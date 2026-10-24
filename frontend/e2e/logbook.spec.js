import { test as base, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createHash } from "node:crypto";

const test = base.extend({
  operatorSession: [async ({ browser }, use, workerInfo) => {
    const context = await browser.newContext({ baseURL: workerInfo.project.use.baseURL });
    try {
      const page = await context.newPage();
      await page.goto("/");
      await page.getByLabel("Nama pengguna", { exact: true }).fill("operator");
      await page.getByLabel("Kata sandi", { exact: true }).fill("frontend-test-only-123");
      await page.getByRole("button", { name: "Masuk", exact: true }).click();
      await expect(page.getByRole("heading", { name: "Pilih mesin", exact: true })).toBeVisible();
      const session = await page.evaluate(() => sessionStorage.getItem("machine-logbook-session"));
      expect(JSON.parse(session).access_token).toBeTruthy();
      await use({ session, origin: new URL(page.url()).origin });
    } finally {
      await context.close();
    }
  }, { scope: "worker" }],
  page: async ({ page, operatorSession }, use) => {
    // sessionStorage is not part of Playwright storageState; seed each isolated tab.
    await page.addInitScript(({ session, origin }) => {
      if (location.origin === origin && !sessionStorage.getItem("machine-logbook-session")) {
        sessionStorage.setItem("machine-logbook-session", session);
      }
    }, operatorSession);
    await use(page);
  },
});

async function login(page) {
  await page.goto("/");
  // Login UI runs once per worker; each test still owns a fresh browser context.
  await expect(page.getByRole("heading", { name: "Pilih mesin", exact: true })).toBeVisible();
}
async function choose(page, name = "MAILENDER 222") {
  await page.getByRole("link", { name, exact: true }).click();
  await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
  await expect(page.getByLabel(`Pesan untuk ${name}`)).toBeVisible();
}
async function accessible(page) {
  expect((await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa"]).analyze()).violations).toEqual([]);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
}
async function photo(page) {
  const data = await page.evaluate(() => {
    const canvas = document.createElement("canvas");
    canvas.width = 80; canvas.height = 80;
    const context = canvas.getContext("2d");
    context.fillStyle = "white"; context.fillRect(0, 0, 80, 80);
    context.fillStyle = "black"; context.fillText("TEST", 10, 30);
    return canvas.toDataURL("image/png").split(",")[1];
  });
  return { name: "test.png", mimeType: "image/png", buffer: Buffer.from(data, "base64") };
}
async function token(page) {
  return page.evaluate(() => JSON.parse(sessionStorage.getItem("machine-logbook-session")).access_token);
}
async function send(page) {
  const response = page.waitForResponse((r) => /\/machines\/\d+\/messages$/.test(new URL(r.url()).pathname) && r.request().method() === "POST");
  await page.getByRole("button", { name: "Kirim pesan", exact: true }).click();
  const result = await response;
  expect(result.status()).toBe(201);
  await expect(page.getByRole("status").filter({ hasText: "Pesan terkirim." })).toBeVisible();
  return result.json();
}

test("photo markers persist through reload and transformed placement", async ({ page }) => {
  await login(page);
  await choose(page);
  await page.locator("#message-photos").setInputFiles(await photo(page));
  const message = await send(page);
  const entry = page.locator(".message-entry").last();
  await entry.locator(".message-photo-open").click();
  const stage = page.locator(".photo-dialog-viewport");
  const markerButton = page.getByRole("button", { name: "Aktifkan marker foto", exact: true });
  await expect(markerButton).toBeEnabled();
  await markerButton.click();
  const image = page.locator(".photo-dialog-image-layer img");
  let box = await image.boundingBox();
  await page.mouse.click(box.x + box.width * 0.25, box.y + box.height * 0.75);
  await expect(page.locator(".photo-dialog-marker")).toHaveCount(1);
  await page.getByRole("button", { name: "Putar foto 90 derajat searah jarum jam", exact: true }).click();
  await page.getByRole("button", { name: "Perbesar foto", exact: true }).click();
  await expect(page.getByLabel("Pembesaran foto", { exact: true })).toHaveText("150%");
  await expect.poll(async () => image.evaluate((el) => getComputedStyle(el.parentElement).transform)).toBe("matrix(0, 1.5, -1.5, 0, 0, 0)");
  box = await image.boundingBox();
  await page.mouse.click(box.x + box.width * 0.6, box.y + box.height * 0.7);
  await expect(page.locator(".photo-dialog-marker")).toHaveCount(2);
  const saved = await page.request.get(`${message.photos[0].url}/markers`, { headers: { Authorization: `Bearer ${await token(page)}` } });
  const { items } = await saved.json();
  expect(items[0].x).toBeCloseTo(0.25, 1);
  expect(items[0].y).toBeCloseTo(0.75, 1);
  expect(items[1].x).toBeCloseTo(0.7, 1);
  expect(items[1].y).toBeCloseTo(0.4, 1);
  await stage.focus();
  const cursor = page.locator(".photo-dialog-marker-cursor");
  const before = await cursor.boundingBox();
  await page.keyboard.press("ArrowRight");
  const after = await cursor.boundingBox();
  expect(after.x).toBeGreaterThan(before.x);
  expect(after.y).toBeCloseTo(before.y, 1);
  await page.keyboard.press("Enter");
  await expect(page.locator(".photo-dialog-marker")).toHaveCount(3);
  await page.reload();
  await page.locator(".message-entry").last().locator(".message-photo-open").click();
  await expect(page.locator(".photo-dialog-marker")).toHaveCount(3);
});

test("inline coating machine is selectable with shared coating photo", async ({ page }) => {
  await login(page);
  const card = page.getByRole("link", { name: "COATING INLINE", exact: true });
  await expect(card).toBeVisible();
  await expect(card.locator("img")).toHaveAttribute("src", /\/machine\/coating\.png$/);
  await choose(page, "COATING INLINE");
  await accessible(page);
});

test("machine conversation persists exact text, photos, both, and historical archives", async ({ page }, testInfo) => {
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await login(page);
  await accessible(page);
  await choose(page);
  await expect(page.getByLabel("Tanggal logbook")).toHaveCount(0);
  await expect(page.getByRole("combobox", { name: "Shift", exact: true })).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Kirim pesan", exact: true })).toBeDisabled();
  const body = `  ${testInfo.project.name} ${Date.now()}\n\n<script>window.bad = true</script>\takhir  `;
  const input = page.getByLabel("Pesan untuk MAILENDER 222");
  await input.fill(body);
  const text = await send(page);
  expect(text.text).toBe(body);
  expect(text.photos).toEqual([]);
  expect(text.author.name).toBe("Pengujian operator");
  await expect(page.locator(".message-text").filter({ hasText: testInfo.project.name }).last()).toHaveJSProperty("textContent", body);
  await expect(page.locator(".message-entry.message-own").filter({ hasText: testInfo.project.name }).last()).toBeVisible();
  expect(await page.evaluate(() => window.bad)).toBeUndefined();
  await expect(input).toHaveValue("");
  const image = await photo(page);
  await expect(page.getByRole("button", { name: "Ambil foto dari kamera" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Tambah foto dari galeri", exact: true })).toBeVisible();
  await page.locator("#message-photos").setInputFiles(image);
  const photosOnly = await send(page);
  expect(photosOnly.text).toBe("");
  expect(photosOnly.photos[0].sha256).toBe(createHash("sha256").update(image.buffer).digest("hex"));
  await input.fill("Catatan dengan foto");
  await page.locator("#message-photos").setInputFiles([image, image]);
  await page.getByRole("button", { name: "Hapus foto 2", exact: true }).click();
  const both = await send(page);
  expect(both.text).toBe("Catatan dengan foto");
  expect(both.photos).toHaveLength(1);
  await page.reload();
  await expect(page.locator(".message-text").filter({ hasText: testInfo.project.name }).last()).toHaveJSProperty("textContent", body);
  const open = page.getByRole("button", { name: "Perbesar foto 1", exact: true }).last();
  await open.click();
  const dialogImg = page.getByRole("dialog").getByRole("img");
  await expect(dialogImg).toBeVisible();
  const rotateBtn = page.getByRole("button", { name: /Putar foto/ });
  await expect(rotateBtn).toBeVisible();
  await rotateBtn.click();
  await expect(dialogImg).toHaveAttribute("style", /rotate\(90deg\)/);
  await page.keyboard.press("r");
  await expect(dialogImg).toHaveAttribute("style", /rotate\(180deg\)/);
  const resetBtn = page.getByRole("button", { name: /Atur ulang/ });
  await resetBtn.click();
  await expect(dialogImg).toHaveAttribute("style", /rotate\(0deg\)/);
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await open.click();
  await page.getByRole("dialog").getByRole("button", { name: "Tutup", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await accessible(page);
  const viewport = page.viewportSize();
  for (const width of [320, 768, 1024]) {
    await page.setViewportSize({ width, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await page.setViewportSize(viewport);
  await page.screenshot({ path: testInfo.outputPath("conversation.png"), fullPage: true });

  const legacyResponse = await page.request.post("/api/v1/logbooks", {
    headers: { Authorization: `Bearer ${await token(page)}` },
    multipart: { machine_id: "1", log_date: testInfo.project.name === "desktop" ? "2025-01-02" : "2025-01-03", shift_id: "1", photos: image },
  });
  expect(legacyResponse.status()).toBe(201);
  const legacy = await legacyResponse.json();
  await page.reload();
  const legacyEntry = page.locator(".message-entry").filter({ has: page.getByRole("link", { name: "Buka arsip lama" }) }).last();
  await expect(legacyEntry.locator(".message-text")).toHaveCount(0);
  await legacyEntry.getByRole("link", { name: "Buka arsip lama" }).click();
  await expect(page).toHaveURL(new RegExp(`/logbooks/${legacy.id}$`));
  await expect(page.getByRole("img", { name: /Logbook .*halaman 1/ })).toBeVisible();
  await expect(page.getByRole("link", { name: "Tambah halaman", exact: true })).toHaveCount(0);
  await page.getByRole("link", { name: "Catatan mesin", exact: true }).click();
  await expect(page).toHaveURL(/\/machines\/1$/);
  expect(errors).toEqual([]);
});

test("failed submission retains machine-scoped text and attachments", async ({ page }) => {
  await login(page); await choose(page);
  const text = "Draft\n  masih ada";
  await page.getByLabel("Pesan untuk MAILENDER 222").fill(text);
  await page.locator("#message-photos").setInputFiles(await photo(page));
  await page.route("**/api/v1/machines/1/messages", (route) => route.request().method() === "POST" ? route.abort("failed") : route.continue());
  await page.getByRole("button", { name: "Kirim pesan", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("Koneksi terputus");
  await expect(page.getByLabel("Pesan untuk MAILENDER 222")).toHaveValue(text);
  await expect(page.getByRole("img", { name: "Pratinjau foto 1" })).toBeVisible();
  await page.getByRole("link", { name: "Pilih mesin lain", exact: true }).click();
  await choose(page, "MS3");
  await expect(page.getByLabel("Pesan untuk MS3")).toHaveValue("");
  await page.getByLabel("Pesan untuk MS3").fill("Draft mesin kedua");
  await page.getByRole("link", { name: "Pilih mesin lain", exact: true }).click();
  await choose(page);
  await expect(page.getByLabel("Pesan untuk MAILENDER 222")).toHaveValue(text);
  await expect(page.getByRole("img", { name: "Pratinjau foto 1" })).toBeVisible();
  await page.unroute("**/api/v1/machines/1/messages");
  await send(page);
  await expect(page.getByLabel("Pesan untuk MAILENDER 222")).toHaveValue("");
  await expect(page.getByRole("img", { name: "Pratinjau foto 1" })).toHaveCount(0);
});

test("loading, empty, failure retry and older pagination preserve ascending order", async ({ page }) => {
  await login(page);
  const message = (id, text) => ({ id, machine_id: 1, text, author: { id: "test-author", name: "Pengujian operator" }, created_at: `2026-01-0${id}T00:00:00Z`, photos: [] });
  let fail = true;
  let release;
  const pending = new Promise((resolve) => { release = resolve; });
  await page.route("**/api/v1/machines/1/messages?*", async (route) => {
    await pending;
    if (fail) return route.fulfill({ status: 500, json: { error: { code: "internal_error" } } });
    const cursor = new URL(route.request().url()).searchParams.get("cursor");
    return route.fulfill({ json: cursor ? { items: [message("1", "Pertama"), message("2", "Kedua")] } : { items: [message("2", "Kedua"), message("3", "Ketiga")], next_cursor: "opaque+/=" } });
  });
  await choose(page);
  await expect(page.getByRole("status").filter({ hasText: "Memuat pesan" })).toBeVisible();
  release();
  await expect(page.getByRole("alert")).toBeVisible();
  fail = false;
  await page.getByRole("button", { name: "Coba lagi", exact: true }).click();
  await expect(page.locator(".message-text")).toHaveText(["Kedua", "Ketiga"]);
  await page.getByRole("button", { name: "Muat pesan sebelumnya", exact: true }).click();
  await expect(page.locator(".message-text")).toHaveText(["Pertama", "Kedua", "Ketiga"]);
  await expect(page.locator(".message-date")).toHaveCount(3);
  await expect(page.getByRole("button", { name: "Muat pesan sebelumnya", exact: true })).toHaveCount(0);
  await page.route("**/api/v1/machines/2/messages?*", (route) => route.fulfill({ json: { items: [] } }));
  await page.getByRole("link", { name: "Pilih mesin lain", exact: true }).click();
  await choose(page, "MS3");
  await expect(page.getByRole("heading", { name: "Belum ada pesan" })).toBeVisible();
  await page.getByLabel("Pesan untuk MS3").fill(" \n\t");
  await expect(page.getByRole("button", { name: "Kirim pesan", exact: true })).toBeDisabled();
  await accessible(page);
});

test("submission stays with its machine during navigation and prevents double send", async ({ page }) => {
  await login(page); await choose(page);
  let release;
  let posts = 0;
  const pending = new Promise((resolve) => { release = resolve; });
  await page.route("**/api/v1/machines/1/messages", async (route) => {
    if (route.request().method() !== "POST") return route.continue();
    posts += 1;
    await pending;
    return route.continue();
  });
  await page.getByLabel("Pesan untuk MAILENDER 222").fill("Pengiriman saat berpindah mesin");
  const response = page.waitForResponse((r) => r.url().endsWith("/machines/1/messages") && r.request().method() === "POST");
  await page.getByRole("button", { name: "Kirim pesan", exact: true }).click();
  await expect(page.getByRole("button", { name: "Mengirim pesan", exact: true })).toBeDisabled();
  await expect(page.getByLabel("Pesan untuk MAILENDER 222")).toBeDisabled();
  await page.getByRole("link", { name: "Pilih mesin lain", exact: true }).click();
  await choose(page, "MS3");
  release();
  expect((await response).status()).toBe(201);
  await expect(page.getByLabel("Pesan untuk MS3")).toHaveValue("");
  await page.getByRole("link", { name: "Pilih mesin lain", exact: true }).click();
  await choose(page);
  await expect(page.getByLabel("Pesan untuk MAILENDER 222")).toHaveValue("");
  await expect(page.locator(".message-text").filter({ hasText: "Pengiriman saat berpindah mesin" }).last()).toBeVisible();
  expect(posts).toBe(1);
});

async function mockCamera(page, { denied = false, deferred = false, width = 640, height = 480 } = {}) {
  await page.addInitScript(({ denied, deferred, width, height }) => {
    window.cameraTest = { streams: [], requests: [], pending: [] };
    Object.defineProperty(navigator.mediaDevices, "getUserMedia", {
      configurable: true,
      value: async (constraints) => {
        window.cameraTest.requests.push(constraints);
        if (denied) throw new DOMException("Permission denied", "NotAllowedError");
        if (deferred) await new Promise((resolve) => window.cameraTest.pending.push(resolve));
        const canvas = document.createElement("canvas");
        canvas.width = width;
        canvas.height = height;
        const context = canvas.getContext("2d");
        const paint = () => {
          context.fillStyle = "white";
          context.fillRect(0, 0, canvas.width, canvas.height);
          context.fillStyle = "black";
          context.font = "24px sans-serif";
          context.fillText("CAMERA TEST", 30, 120);
        };
        paint();
        const stream = canvas.captureStream(10);
        const timer = setInterval(paint, 100);
        const track = stream.getVideoTracks()[0];
        const stop = track.stop.bind(track);
        track.stop = () => { clearInterval(timer); stop(); };
        window.cameraTest.streams.push(stream);
        return stream;
      },
    });
  }, { denied, deferred, width, height });
}

async function openCamera(page) {
  await page.getByRole("button", { name: "Ambil foto dari kamera", exact: true }).click();
  const camera = page.getByRole("dialog", { name: "Kamera", exact: true });
  await expect(camera).toBeVisible();
  return camera;
}

async function expectCameraStopped(page) {
  await expect.poll(() => page.evaluate(() => window.cameraTest.streams.length)).toBeGreaterThan(0);
  await expect.poll(() => page.evaluate(() => window.cameraTest.streams.every((stream) =>
    stream.getTracks().every((track) => track.readyState === "ended")))).toBe(true);
}

test("camera captures multiple full-size frames, reviews and removes photos, then returns shared draft", async ({ page }) => {
  await mockCamera(page, { width: 2880, height: 1620 });
  await login(page); await choose(page);
  const gallery = await photo(page);
  await page.locator("#message-photos").setInputFiles(gallery);
  await page.getByLabel("Pesan untuk MAILENDER 222").fill("Draft kamera");
  const gallerySource = await page.getByRole("img", { name: "Pratinjau foto 1", exact: true }).getAttribute("src");
  const camera = await openCamera(page);
  const shutter = camera.getByRole("button", { name: "Ambil foto", exact: true });
  await expect(camera.locator("video")).toHaveJSProperty("videoWidth", 2880);
  await expect(camera.locator("video")).toHaveJSProperty("videoHeight", 1620);
  await expect(camera.locator("video")).toHaveCSS("object-fit", "contain");
  for (const count of [2, 3]) {
    await expect(shutter).toBeEnabled();
    await shutter.click();
    await expect(camera.getByRole("button", { name: `Selesai (${count} foto)`, exact: true })).toBeVisible();
    await expect(camera.getByRole("button", { name: /^Lihat foto \d+$/ })).toHaveCount(count);
  }
  await camera.getByRole("button", { name: "Lihat foto 2", exact: true }).click();
  await expect(camera.getByRole("button", { name: "Hapus foto 2", exact: true })).toBeVisible();
  const capturedSource = await camera.getByRole("button", { name: "Lihat foto 2", exact: true }).locator("img").getAttribute("src");
  expect(await page.evaluate(async (src) => {
    const image = new Image();
    image.src = src;
    await image.decode();
    return [image.naturalWidth, image.naturalHeight];
  }, capturedSource)).toEqual([2880, 1620]);
  await camera.getByRole("button", { name: "Kembali ke kamera", exact: true }).click();
  await expect(shutter).toBeEnabled();
  await camera.getByRole("button", { name: "Lihat foto 3", exact: true }).click();
  await camera.getByRole("button", { name: "Hapus foto 3", exact: true }).click();
  await expect(camera.getByRole("button", { name: /^Lihat foto \d+$/ })).toHaveCount(2);
  await camera.getByRole("button", { name: "Selesai (2 foto)", exact: true }).click();
  await expect(camera).toHaveCount(0);
  await expectCameraStopped(page);
  await expect(page.getByLabel("Pesan untuk MAILENDER 222")).toHaveValue("Draft kamera");
  await expect(page.getByRole("img", { name: "Pratinjau foto 1", exact: true })).toHaveAttribute("src", gallerySource);
  await expect(page.getByRole("img", { name: "Pratinjau foto 2", exact: true })).toHaveAttribute("src", capturedSource);
  const message = await send(page);
  expect(message.text).toBe("Draft kamera");
  expect(message.photos).toHaveLength(2);
  expect(message.photos[0].sha256).toBe(createHash("sha256").update(gallery.buffer).digest("hex"));
  const outgoing = page.locator(".message-entry.message-own").last();
  const image = outgoing.getByRole("button", { name: "Perbesar foto 2", exact: true }).getByRole("img");
  await expect(image).toBeVisible();
  await expect.poll(() => image.evaluate((node) => node.complete && node.naturalWidth > 0)).toBe(true);
  await expect(page.getByRole("img", { name: /^Pratinjau foto/ })).toHaveCount(0);
  await page.reload();
  await expect(image).toBeVisible();
  await expect.poll(() => image.evaluate((node) => node.complete && node.naturalWidth > 0)).toBe(true);
});

test("camera close and Escape retain captures; switching and reopening release old streams", async ({ page }) => {
  await mockCamera(page);
  await login(page); await choose(page);
  let camera = await openCamera(page);
  const shutter = camera.getByRole("button", { name: "Ambil foto", exact: true });
  await expect(shutter).toBeEnabled();
  await shutter.click();
  await expect(camera.getByRole("button", { name: "Selesai (1 foto)", exact: true })).toBeVisible();
  const beforeSwitch = await page.evaluate(() => window.cameraTest.streams.length);
  await camera.getByRole("button", { name: "Ganti kamera", exact: true }).click();
  await expect.poll(() => page.evaluate(() => window.cameraTest.streams.length)).toBeGreaterThan(beforeSwitch);
  await expect.poll(() => page.evaluate(() => window.cameraTest.streams.slice(0, -1).every((stream) =>
    stream.getTracks().every((track) => track.readyState === "ended")))).toBe(true);
  await expect(shutter).toBeEnabled();
  await camera.getByRole("button", { name: "Tutup dialog", exact: true }).click();
  await expect(camera).toHaveCount(0);
  await expectCameraStopped(page);
  await expect(page.getByRole("img", { name: "Pratinjau foto 1", exact: true })).toBeVisible();
  camera = await openCamera(page);
  await expect(camera.getByRole("button", { name: "Lihat foto 1", exact: true })).toBeVisible();
  await expect(shutter).toBeEnabled();
  await shutter.click();
  await expect(camera.getByRole("button", { name: "Selesai (2 foto)", exact: true })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(camera).toHaveCount(0);
  await expectCameraStopped(page);
  await expect(page.getByRole("img", { name: /^Pratinjau foto/ })).toHaveCount(2);
  camera = await openCamera(page);
  await expect(camera.getByRole("button", { name: /^Lihat foto \d+$/ })).toHaveCount(2);
  await camera.getByRole("button", { name: "Selesai (2 foto)", exact: true }).click();
  await expectCameraStopped(page);
});

test("denied camera permission still allows gallery selection into shared draft", async ({ page }) => {
  await mockCamera(page, { denied: true });
  await login(page); await choose(page);
  const gallery = await photo(page);
  const camera = await openCamera(page);
  await expect(camera.getByRole("alert")).toBeVisible();
  await expect(camera.getByRole("button", { name: "Ambil foto", exact: true })).toBeDisabled();
  const chooser = page.waitForEvent("filechooser");
  await camera.getByRole("button", { name: "Pilih dari galeri", exact: true }).click();
  await (await chooser).setFiles(gallery);
  await expect(camera.getByRole("button", { name: "Selesai (1 foto)", exact: true })).toBeVisible();
  await camera.getByRole("button", { name: "Selesai (1 foto)", exact: true }).click();
  await expect(camera).toHaveCount(0);
  await expect(page.getByRole("img", { name: "Pratinjau foto 1", exact: true })).toBeVisible();
});

test("camera shares twenty-photo limit and keeps shutter visible without vertical dialog scrolling", async ({ page }) => {
  await mockCamera(page);
  await login(page); await choose(page);
  const gallery = await photo(page);
  await page.locator("#message-photos").setInputFiles(Array.from({ length: 19 }, (_, index) => ({ ...gallery, name: `gallery-${index}.png` })));
  const camera = await openCamera(page);
  const shutter = camera.getByRole("button", { name: "Ambil foto", exact: true });
  await expect(shutter).toBeEnabled();
  await shutter.click();
  await expect(camera.getByRole("button", { name: /^Lihat foto \d+$/ })).toHaveCount(20);
  await expect(shutter).toBeDisabled();
  await expect(camera.getByRole("button", { name: "Pilih dari galeri", exact: true })).toBeDisabled();
  for (const viewport of [{ width: 320, height: 568 }, { width: 667, height: 375 }, { width: 1440, height: 1000 }]) {
    await page.setViewportSize(viewport);
    await expect(shutter).toBeInViewport({ ratio: 1 });
    await expect(camera.getByRole("button", { name: "Selesai (20 foto)", exact: true })).toBeInViewport({ ratio: 1 });
    await expect.poll(() => camera.evaluate((node) => node.scrollHeight <= node.clientHeight + 1)).toBe(true);
    expect(await camera.evaluate((node) => {
      const box = node.getBoundingClientRect();
      return box.top >= 0 && box.bottom <= innerHeight + 1 && box.left >= 0 && box.right <= innerWidth + 1;
    })).toBe(true);
  }
  await camera.getByRole("button", { name: "Lihat foto 20", exact: true }).click();
  await camera.getByRole("button", { name: "Hapus foto 20", exact: true }).click();
  await expect(camera.getByRole("button", { name: /^Lihat foto \d+$/ })).toHaveCount(19);
  const back = camera.getByRole("button", { name: "Kembali ke kamera", exact: true });
  if (await back.isVisible()) await back.click();
  await expect(shutter).toBeEnabled();
  await shutter.click();
  await expect(camera.getByRole("button", { name: "Selesai (20 foto)", exact: true })).toBeVisible();
  await camera.getByRole("button", { name: "Selesai (20 foto)", exact: true }).click();
  await expect(page.getByRole("img", { name: /^Pratinjau foto/ })).toHaveCount(20);
  await expectCameraStopped(page);
});

test("camera stops streams returned after permission resolves on a closed dialog", async ({ page }) => {
  await mockCamera(page, { deferred: true });
  await login(page); await choose(page);
  const camera = await openCamera(page);
  await expect.poll(() => page.evaluate(() => window.cameraTest.pending.length)).toBeGreaterThan(0);
  await expect(camera.getByRole("button", { name: "Ambil foto", exact: true })).toBeDisabled();
  await camera.getByRole("button", { name: "Tutup dialog", exact: true }).click();
  await expect(camera).toHaveCount(0);
  await page.evaluate(() => window.cameraTest.pending.splice(0).forEach((resolve) => resolve()));
  await expectCameraStopped(page);
  await expect(page.getByRole("img", { name: /^Pratinjau foto/ })).toHaveCount(0);
});

test("long chat scrolls its timeline while header and composer stay in viewport", async ({ page }) => {
  await login(page);
  const items = Array.from({ length: 40 }, (_, index) => ({
    id: String(index + 1),
    machine_id: 1,
    text: `Catatan ${index + 1}\nPemeriksaan mesin\nHasil pemeriksaan tercatat`,
    author: { id: "other-operator", name: "Operator lain" },
    created_at: `2026-01-${index < 20 ? "01" : "02"}T08:${String(index % 20).padStart(2, "0")}:00Z`,
    photos: [],
  }));
  await page.route("**/api/v1/machines/1/messages?*", (route) => route.fulfill({ json: { items } }));
  await choose(page);
  const timeline = page.getByRole("region", { name: "Riwayat pesan", exact: true });
  await expect(page.locator(".message-entry")).toHaveCount(40);
  await expect(page.locator(".message-own")).toHaveCount(0);
  await expect(page.locator(".message-date")).toHaveCount(2);
  await expect.poll(() => timeline.evaluate((node) => node.scrollHeight > node.clientHeight && node.scrollTop > 0)).toBe(true);
  for (const position of ["top", "bottom"]) {
    await timeline.evaluate((node, edge) => { node.scrollTop = edge === "top" ? 0 : node.scrollHeight; }, position);
    await expect(page.locator(".message-entry")[position === "top" ? "first" : "last"]()).toBeInViewport();
    await expect(page.getByRole("heading", { name: "MAILENDER 222", exact: true })).toBeInViewport();
    await expect(page.getByLabel("Pesan untuk MAILENDER 222")).toBeInViewport();
    await expect(page.getByRole("button", { name: "Kirim pesan", exact: true })).toBeInViewport();
    expect(await page.evaluate(() => document.scrollingElement.scrollHeight <= innerHeight + 1)).toBe(true);
  }
});

test("search finds full machine history, paginates matches and survives navigation", async ({ page }, testInfo) => {
  await login(page);
  const headers = { Authorization: `Bearer ${await token(page)}` };
  const term = `search-${testInfo.project.name}-${Date.now()}`;
  const literalQuery = `${term} oldest 100%_literal`;
  async function post(machine, text, image) {
    const response = await page.request.post(`/api/v1/machines/${machine}/messages`, {
      headers, multipart: image ? { text, photos: image } : { text },
    });
    expect(response.status()).toBe(201);
    return response.json();
  }
  const oldest = literalQuery;
  await post(1, oldest, await photo(page));
  for (let i = 0; i < 21; i += 1) await post(1, `${term} match ${i}`);
  for (let i = 0; i < 21; i += 1) await post(1, `newer unrelated ${i}`);
  await post(2, `${term} other machine`);
  await choose(page);
  await expect(page.locator(".message-text").filter({ hasText: term })).toHaveCount(0);
  const composer = page.getByLabel("Pesan untuk MAILENDER 222");
  await composer.fill("Draft tetap ada");
  await page.locator("#message-photos").setInputFiles(await photo(page));
  await page.getByRole("button", { name: "Cari pesan", exact: true }).click();
  const search = page.getByRole("searchbox", { name: "Cari teks di mesin ini" });
  await expect(search).toBeFocused();
  await search.fill(`  ${term.toUpperCase()}  `);
  await expect(page).not.toHaveURL(/\?q=/);
  await page.getByRole("button", { name: "Cari", exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`q=${term.toUpperCase()}`));
  await expect(page.locator(".message-text")).toHaveCount(20);
  await expect(composer).toHaveValue("Draft tetap ada");
  await expect(page.getByRole("img", { name: "Pratinjau foto 1" })).toBeVisible();
  await page.getByRole("button", { name: "Muat hasil sebelumnya", exact: true }).click();
  await expect(page.locator(".message-text")).toHaveCount(22);
  await expect(page.locator(".message-text").first()).toHaveText(oldest);
  await expect(page.locator(".message-text").filter({ hasText: "other machine" })).toHaveCount(0);
  const oldestEntry = page.locator(".message-entry").filter({ hasText: oldest });
  await oldestEntry.getByRole("button", { name: "Perbesar foto 1", exact: true }).click();
  await expect(page.getByRole("dialog").getByRole("img")).toBeVisible();
  await page.keyboard.press("Escape");
  await accessible(page);
  const viewport = page.viewportSize();
  for (const width of [320, 768, 1024]) {
    await page.setViewportSize({ width, height: 900 });
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await page.setViewportSize(viewport);
  await page.getByRole("button", { name: "Hapus foto 1", exact: true }).click();
  await composer.fill("Sent outside active filter");
  const refreshed = page.waitForResponse((response) => response.request().method() === "GET" && new URL(response.url()).searchParams.get("q") === term.toUpperCase());
  await send(page);
  await refreshed;
  await expect(page.locator(".message-text")).toHaveCount(20);
  await expect(page.locator(".message-text").filter({ hasText: "Sent outside active filter" })).toHaveCount(0);
  await search.fill(literalQuery);
  await search.press("Enter");
  await expect(page.locator(".message-text")).toHaveText([oldest]);
  await page.reload();
  await expect(search).toHaveValue(literalQuery);
  await expect(page.locator(".message-text")).toHaveText([oldest]);
  await page.getByRole("button", { name: "Reset", exact: true }).click();
  await expect(page).toHaveURL(/\/machines\/1$/);
  await expect(page.locator(".message-text").filter({ hasText: "Sent outside active filter" })).toBeVisible();
  await page.goBack();
  await expect(search).toHaveValue(literalQuery);
  await expect(page.locator(".message-text")).toHaveText([oldest]);
  await page.goForward();
  await expect(search).toHaveValue("");
  await search.fill(`${term}-missing`);
  await search.press("Enter");
  await expect(page.getByRole("heading", { name: "Tidak ada pesan yang cocok" })).toBeVisible();
  await search.fill("x".repeat(201));
  await search.press("Enter");
  await expect(page.getByRole("alert")).toContainText("maksimal 200 karakter");
  await search.press("Escape");
  await expect(search).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Cari pesan", exact: true })).toBeFocused();
  await page.getByRole("link", { name: "Pilih mesin lain", exact: true }).click();
  await choose(page, "MS3");
  await page.getByRole("button", { name: "Cari pesan", exact: true }).click();
  await search.fill(term);
  await search.press("Enter");
  await expect(page.locator(".message-text")).toHaveText([`${term} other machine`]);
});

test("search cancels stale pages and retries failures without losing the query", async ({ page }) => {
  await login(page);
  const message = (id, text) => ({ id, machine_id: 1, text, author: { id: "search-author", name: "Pengujian operator" }, created_at: "2026-01-01T00:00:00Z", photos: [] });
  let releaseInitial;
  let releaseOlder;
  let olderStarted;
  const initial = new Promise((resolve) => { releaseInitial = resolve; });
  const older = new Promise((resolve) => { releaseOlder = resolve; });
  const started = new Promise((resolve) => { olderStarted = resolve; });
  let fail = true;
  await page.route("**/api/v1/machines/1/messages?*", async (route) => {
    const params = new URL(route.request().url()).searchParams;
    const q = params.get("q");
    if (!q) {
      await initial;
      return route.fulfill({ json: { items: [message("old-initial", "Stale initial")] } });
    }
    if (q === "first" && params.has("cursor")) {
      olderStarted();
      await older;
      return route.fulfill({ json: { items: [message("old-page", "Stale older")] } });
    }
    if (q === "broken" && fail) return route.fulfill({ status: 500, json: { error: { code: "internal_error" } } });
    return route.fulfill({ json: { items: [message(q, q)], ...(q === "first" ? { next_cursor: "first-cursor" } : {}) } });
  });
  await choose(page);
  await page.getByRole("button", { name: "Cari pesan", exact: true }).click();
  const search = page.getByRole("searchbox", { name: "Cari teks di mesin ini" });
  await search.fill("first");
  await search.press("Enter");
  await expect(page.locator(".message-text")).toHaveText(["first"]);
  releaseInitial();
  await page.getByRole("button", { name: "Muat hasil sebelumnya", exact: true }).click();
  await started;
  await search.fill("second");
  await search.press("Enter");
  await expect(page.locator(".message-text")).toHaveText(["second"]);
  releaseOlder();
  await search.fill("first");
  await search.press("Enter");
  await expect(page.getByRole("button", { name: "Muat hasil sebelumnya", exact: true })).toBeEnabled();
  await page.getByRole("button", { name: "Muat hasil sebelumnya", exact: true }).click();
  await expect(page.locator(".message-text")).toHaveText(["Stale older", "first"]);
  await search.fill("broken");
  await search.press("Enter");
  await expect(page.getByRole("alert")).toBeVisible();
  await expect(search).toHaveValue("broken");
  fail = false;
  await page.getByRole("button", { name: "Coba lagi", exact: true }).click();
  await expect(page.locator(".message-text")).toHaveText(["broken"]);
  await expect(page.locator(".message-text").filter({ hasText: "Stale initial" })).toHaveCount(0);
});

test("message edits preserve photos, order and composer draft across save and reload", async ({ page }) => {
  await login(page);
  await choose(page);
  const initialText = `Catatan sebelum diedit ${Date.now()}`;
  const updatedText = `Catatan sesudah diedit ${Date.now()}`;
  await page.getByLabel("Pesan untuk MAILENDER 222").fill(initialText);
  await page.locator("#message-photos").setInputFiles(await photo(page));
  const original = await send(page);
  const row = page.locator(".message-entry").filter({ hasText: initialText });
  const trigger = row.getByRole("button", { name: /^Edit pesan Anda/ });
  await page.getByLabel("Pesan untuk MAILENDER 222").fill("Draft terpisah");
  await page.locator("#message-photos").setInputFiles(await photo(page));
  await trigger.click();
  const dialog = page.getByRole("dialog", { name: "Edit pesan", exact: true });
  const input = dialog.getByLabel("Teks pesan", { exact: true });
  await expect(input).toHaveValue(original.text);
  await expect(input).toBeFocused();
  await input.fill("Batal perubahan");
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(trigger).toBeFocused();
  await expect(row.locator(".message-text")).toHaveText(original.text);
  await trigger.click();
  await dialog.getByRole("button", { name: "Simpan perubahan" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(row.getByText("Diedit", { exact: true })).toHaveCount(0);
  await trigger.click();
  await input.fill(updatedText);
  let release;
  const pending = new Promise((resolve) => { release = resolve; });
  await page.route(`**/api/v1/machines/1/messages/${original.id}`, async (route) => {
    if (route.request().method() !== "PATCH") return route.continue();
    expect(route.request().postDataJSON()).toEqual({ text: updatedText });
    await pending;
    await route.continue();
  });
  const response = page.waitForResponse((r) => r.request().method() === "PATCH" && r.url().endsWith(`/messages/${original.id}`));
  await dialog.getByRole("button", { name: "Simpan perubahan" }).click();
  await expect(dialog.getByRole("button", { name: "Menyimpan…" })).toBeDisabled();
  await expect(input).toBeDisabled();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeVisible();
  release();
  const savedResponse = await response;
  expect(savedResponse.status()).toBe(200);
  const saved = await savedResponse.json();
  expect(saved.created_at).toBe(original.created_at);
  expect(saved.photos).toEqual(original.photos);
  expect(saved.edited_at).toBeTruthy();
  await expect(dialog).toHaveCount(0);
  const editedRow = page.locator(".message-entry").filter({ hasText: updatedText });
  await expect(editedRow.getByText("Diedit", { exact: true })).toBeVisible();
  await expect(page.getByLabel("Pesan untuk MAILENDER 222")).toHaveValue("Draft terpisah");
  await expect(page.getByRole("img", { name: "Pratinjau foto 1" })).toBeVisible();
  await accessible(page);
  await page.reload();
  await expect(editedRow.getByText("Diedit", { exact: true })).toBeVisible();
  await editedRow.getByRole("button", { name: /^Edit pesan Anda/ }).click();
  await input.fill("");
  const clearedResponse = page.waitForResponse((r) => r.request().method() === "PATCH" && r.url().endsWith(`/messages/${original.id}`));
  await page.unroute(`**/api/v1/machines/1/messages/${original.id}`);
  await dialog.getByRole("button", { name: "Simpan perubahan" }).click();
  const cleared = await (await clearedResponse).json();
  expect(cleared.text).toBe("");
  expect(cleared.photos).toEqual(original.photos);
});

test("message edit validation and failed save retain text without changing saved content", async ({ page }) => {
  await login(page);
  await choose(page);
  await page.getByLabel("Pesan untuk MAILENDER 222").fill(`Pesan validasi edit ${Date.now()}`);
  const original = await send(page);
  const row = page.locator(".message-entry").filter({ hasText: original.text });
  await row.getByRole("button", { name: /^Edit pesan Anda/ }).click();
  const dialog = page.getByRole("dialog", { name: "Edit pesan", exact: true });
  const input = dialog.getByLabel("Teks pesan", { exact: true });
  await input.fill(" \n");
  await dialog.getByRole("button", { name: "Simpan perubahan" }).click();
  await expect(dialog.getByRole("alert")).toContainText("Tulis pesan");
  await input.fill("a".repeat(20_001));
  await dialog.getByRole("button", { name: "Simpan perubahan" }).click();
  await expect(dialog.getByRole("alert")).toContainText("20.000");
  await input.fill("Perubahan belum tersimpan");
  await page.route(`**/api/v1/machines/1/messages/${original.id}`, (route) => route.request().method() === "PATCH" ? route.fulfill({ status: 403, json: { error: { code: "forbidden" } } }) : route.continue());
  await dialog.getByRole("button", { name: "Simpan perubahan" }).click();
  await expect(dialog.getByRole("alert")).toContainText("tidak memiliki izin");
  await expect(input).toHaveValue("Perubahan belum tersimpan");
  await expect(row.locator(".message-text")).toHaveText(original.text);
  await expect(row.getByText("Diedit", { exact: true })).toHaveCount(0);
  await dialog.getByRole("button", { name: "Batal", exact: true }).click();
  await expect(row.getByRole("button", { name: /^Edit pesan Anda/ })).toBeFocused();
  await row.getByRole("button", { name: /^Edit pesan Anda/ }).click();
  const retryText = `Perubahan dicoba lagi ${Date.now()}`;
  await input.fill(retryText);
  await dialog.getByRole("button", { name: "Simpan perubahan" }).click();
  await expect(dialog.getByRole("alert")).toContainText("tidak memiliki izin");
  await page.unroute(`**/api/v1/machines/1/messages/${original.id}`);
  await dialog.getByRole("button", { name: "Simpan perubahan" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.locator(".message-entry").filter({ hasText: retryText }).getByText("Diedit", { exact: true })).toBeVisible();
});

test("edit action is restricted to owned native messages", async ({ page }) => {
  await login(page);
  await choose(page);
  await page.getByLabel("Pesan untuk MAILENDER 222").fill(`Pesan pemilik ${Date.now()}`);
  const own = await send(page);
  const items = [
    own,
    { ...own, id: "other-message", text: "Pesan operator lain", author: { id: "other-author", name: "Operator lain" } },
    { ...own, id: "legacy-message", text: "Arsip milik sendiri", legacy_archive: { id: "legacy", log_date: "2026-01-01", shift: { name: "Pagi" } } },
  ];
  await page.route("**/api/v1/machines/1/messages?*", (route) => route.fulfill({ json: { items } }));
  await page.reload();
  await expect(page.locator(".message-entry")).toHaveCount(3);
  await expect(page.getByRole("button", { name: /^Edit pesan Anda/ })).toHaveCount(1);
  await expect(page.locator(".message-entry").filter({ hasText: own.text }).getByRole("button", { name: /^Edit pesan Anda/ })).toBeVisible();
});

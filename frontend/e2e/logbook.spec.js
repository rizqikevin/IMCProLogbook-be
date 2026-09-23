import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createHash } from "node:crypto";

async function login(page, username = "operator") {
  await page.goto("/login");
  await page.getByLabel("Nama pengguna", { exact: true }).fill(username);
  await page
    .getByLabel("Kata sandi", { exact: true })
    .fill("frontend-test-only-123");
  await page.getByRole("button", { name: "Masuk ke arsip" }).click();
  await expect(
    page.getByRole("heading", { name: "Arsip logbook", exact: true }),
  ).toBeVisible();
}

async function noOverflow(page) {
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
}

async function accessible(page) {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21aa"])
    .analyze();
  expect(results.violations).toEqual([]);
}

async function photo(page, label) {
  const data = await page.evaluate((text) => {
    const canvas = document.createElement("canvas");
    canvas.width = 600;
    canvas.height = 800;
    const ctx = canvas.getContext("2d");
    ctx.fillStyle = "#fff";
    ctx.fillRect(0, 0, 600, 800);
    ctx.fillStyle = "#222";
    ctx.font = "24px sans-serif";
    ctx.fillText("FOTO PENGUJIAN", 40, 65);
    ctx.fillText(text, 40, 110);
    ctx.strokeStyle = "#999";
    for (let y = 160; y < 760; y += 55) {
      ctx.beginPath();
      ctx.moveTo(40, y);
      ctx.lineTo(560, y);
      ctx.stroke();
    }
    return canvas.toDataURL("image/png").split(",")[1];
  }, label);
  return {
    name: `${label}.png`,
    mimeType: "image/png",
    buffer: Buffer.from(data, "base64"),
  };
}

test("operator captures, orders, uploads, reads, appends; admin deletes", async ({
  page,
}, testInfo) => {
  const jsErrors = [];
  page.on("pageerror", (error) => jsErrors.push(error.message));
  await page.goto("/login");
  await accessible(page);
  await noOverflow(page);
  await page.screenshot({
    path: testInfo.outputPath("login.png"),
    fullPage: true,
  });
  await page.getByLabel("Nama pengguna", { exact: true }).fill("operator");
  await page.getByLabel("Kata sandi", { exact: true }).fill("wrong-password");
  await page.getByRole("button", { name: "Masuk ke arsip" }).click();
  await expect(page.getByRole("alert")).toContainText(
    "Nama pengguna atau kata sandi salah",
  );
  await login(page);
  await expect(
    page.getByText("Halaman pertama dimulai di sini."),
  ).toBeVisible();
  await accessible(page);
  await page.getByRole("link", { name: "Buat arsip pertama" }).click();
  await expect(
    page.getByRole("heading", { name: "Arsipkan logbook", exact: true }),
  ).toBeVisible();
  await page
    .getByRole("combobox", { name: "Mesin", exact: true })
    .selectOption("1");
  await expect(
    page.getByRole("combobox", { name: "Mesin", exact: true }),
  ).toHaveValue("1");
  await page
    .getByLabel("Tanggal logbook")
    .fill(testInfo.project.name === "desktop" ? "2026-09-23" : "2026-09-24");
  await page
    .getByRole("combobox", { name: "Shift", exact: true })
    .selectOption("2");
  await expect(
    page.getByRole("combobox", { name: "Mesin", exact: true }),
  ).toHaveValue("1");
  const first = await photo(page, "Halaman A");
  const second = await photo(page, "Halaman B");
  let uploads = 0;
  page.on("request", (req) => {
    if (req.method() === "POST" && req.url().includes("/api/v1/logbooks"))
      uploads += 1;
  });
  await page
    .getByLabel("Pilih foto logbook", { exact: true })
    .setInputFiles([first, second]);
  await expect(page.getByText("2 halaman siap disimpan")).toBeVisible();
  await expect(
    page.getByRole("combobox", { name: "Mesin", exact: true }),
  ).toHaveValue("1");
  expect(uploads).toBe(0);
  await page
    .getByRole("button", { name: "Pindahkan foto 2 lebih awal" })
    .click();
  await page.getByRole("link", { name: "Semua arsip", exact: true }).click();
  await expect(page.getByRole("dialog")).toContainText(
    "Tinggalkan foto pilihan?",
  );
  await page.getByRole("button", { name: "Lanjutkan mengarsipkan" }).click();
  await expect(
    page.getByRole("combobox", { name: "Mesin", exact: true }),
  ).toHaveValue("1");
  await accessible(page);
  await noOverflow(page);
  await page.screenshot({
    path: testInfo.outputPath("capture.png"),
    fullPage: true,
  });
  const originalViewport = page.viewportSize();
  for (const width of [320, 768, 1024]) {
    await page.setViewportSize({ width, height: 900 });
    await noOverflow(page);
  }
  await page.setViewportSize(originalViewport);
  await page.getByRole("button", { name: "Done · Simpan arsip" }).click();
  await expect(
    page.getByRole("heading", { name: "MAILENDER 222", exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("status")).toContainText(
    "Arsip berhasil disimpan",
  );
  expect(uploads).toBe(1);
  const bookPath = new URL(page.url()).pathname;
  const book = await page.evaluate(async (path) => {
    const { access_token } = JSON.parse(
      sessionStorage.getItem("machine-logbook-session"),
    );
    return fetch(`/api/v1${path}`, {
      headers: { Authorization: `Bearer ${access_token}` },
    }).then((r) => r.json());
  }, bookPath);
  expect(book.photos[0].sha256).toBe(
    createHash("sha256").update(second.buffer).digest("hex"),
  );
  await expect(page.getByRole("img", { name: /halaman 1/ })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Hapus arsip", exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Berikutnya", exact: true }).click();
  await expect(page.getByRole("img", { name: /halaman 2/ })).toBeVisible();
  await page.getByRole("button", { name: "Perbesar", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await accessible(page);
  await noOverflow(page);
  await page.screenshot({
    path: testInfo.outputPath("detail.png"),
    fullPage: true,
  });
  await page.getByRole("link", { name: "Tambah halaman" }).click();
  await page.getByRole("button", { name: "Buka kamera" }).click();
  const camera = page.getByRole("dialog");
  await expect(
    camera.getByRole("button", { name: "Ambil foto", exact: true }),
  ).toBeEnabled();
  await camera.getByRole("button", { name: "Ambil foto", exact: true }).click();
  await expect(camera.getByText("1 foto dipilih")).toBeVisible();
  await camera.getByRole("button", { name: "Ambil foto", exact: true }).click();
  await expect(camera.getByText("2 foto dipilih")).toBeVisible();
  expect(uploads).toBe(1);
  await page.evaluate(() => {
    window.testCameraTrack = document
      .querySelector("video")
      .srcObject.getTracks()[0];
  });
  await camera.getByRole("button", { name: "Selesai memotret" }).click();
  expect(await page.evaluate(() => window.testCameraTrack.readyState)).toBe(
    "ended",
  );
  await page.getByRole("button", { name: "Hapus foto 2", exact: true }).click();
  await page.route(
    "**/api/v1/logbooks/*/photos",
    async (route) => {
      await route.fetch();
      await route.abort("failed");
    },
    { times: 1 },
  );
  await page.getByRole("button", { name: "Done · Simpan arsip" }).click();
  await expect(
    page.getByText("Periksa arsip sebelum mengunggah ulang", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Done · Simpan arsip" }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Periksa hasil unggahan" }).click();
  await expect(page.getByText("3 halaman logbook")).toBeVisible();
  expect(uploads).toBe(2);
  await page.reload();
  await expect(page.getByText("3 halaman logbook")).toBeVisible();
  await page.getByRole("link", { name: "Semua arsip", exact: true }).click();
  await expect(
    page.getByRole("link", { name: /Buka arsip MAILENDER/ }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("archives.png"),
    fullPage: true,
  });
  await page.getByLabel("Dari tanggal").fill("2026-01-01");
  await page.getByLabel("Sampai tanggal").fill("2026-01-31");
  await expect(page.getByText("Belum ada arsip yang cocok.")).toBeVisible();
  await page.getByRole("button", { name: "Hapus filter", exact: true }).click();
  await expect(
    page.getByRole("link", { name: /Buka arsip MAILENDER/ }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Keluar dari akun" }).click();
  await expect(
    page.getByRole("heading", { name: "Selamat datang kembali." }),
  ).toBeVisible();
  await login(page, "admin");
  await page.goto(bookPath);
  await page.getByRole("button", { name: "Hapus arsip", exact: true }).click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Hapus arsip", exact: true })
    .click();
  await expect(
    page.getByText("Halaman pertama dimulai di sini."),
  ).toBeVisible();
  expect(jsErrors).toEqual([]);
});

test("validation, network errors, keyboard and expired-session handling", async ({
  page,
}) => {
  await login(page);
  await page.getByRole("link", { name: "Buat arsip pertama" }).click();
  await page.getByLabel("Pilih foto logbook", { exact: true }).setInputFiles({
    name: "bad.png",
    mimeType: "image/png",
    buffer: Buffer.from("not an image"),
  });
  await expect(page.getByRole("alert")).toContainText("tidak dapat dibaca");
  await page
    .getByLabel("Pilih foto logbook", { exact: true })
    .setInputFiles(await photo(page, "Draft belum disimpan"));
  await expect(page.getByText("1 halaman siap disimpan")).toBeVisible();
  await page.getByRole("link", { name: "Semua arsip", exact: true }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await page.getByRole("link", { name: "Semua arsip", exact: true }).click();
  await page
    .getByRole("button", { name: "Tinggalkan halaman", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Arsip logbook", exact: true }),
  ).toBeVisible();
  await page.route("**/api/v1/logbooks?**", (route) =>
    route.fulfill({
      status: 503,
      contentType: "application/json",
      body: JSON.stringify({ error: { code: "internal_error" } }),
    }),
  );
  await page.reload();
  await expect(page.getByRole("alert")).toContainText("Server belum dapat");
  await page.unroute("**/api/v1/logbooks?**");
  await page.getByRole("button", { name: "Coba lagi", exact: true }).click();
  await expect(
    page.getByText("Halaman pertama dimulai di sini."),
  ).toBeVisible();
  await page.keyboard.press("Tab");
  expect(
    await page.evaluate(() => document.activeElement !== document.body),
  ).toBe(true);
  await page.route("**/api/v1/auth/me", (route) =>
    route.fulfill({
      status: 401,
      contentType: "application/json",
      body: JSON.stringify({ error: { code: "unauthorized" } }),
    }),
  );
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Masuk ke arsip" }),
  ).toBeVisible();
  expect(
    await page.evaluate(() =>
      sessionStorage.getItem("machine-logbook-session"),
    ),
  ).toBeNull();
});

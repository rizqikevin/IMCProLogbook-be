import { test as base, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

const test = base.extend({
  operatorSession: [async ({ browser }, use, workerInfo) => {
    const context = await browser.newContext({ baseURL: workerInfo.project.use.baseURL, permissions: ["notifications"] });
    try {
      const page = await context.newPage();
      await page.goto("/");
      await expect(page.getByRole("dialog")).toHaveCount(0);
      await page.getByLabel("Nama pengguna", { exact: true }).fill("operator");
      await page.getByLabel("Kata sandi", { exact: true }).fill("frontend-test-only-123");
      await page.getByRole("button", { name: "Masuk", exact: true }).click();
      await expect(page.getByRole("heading", { name: "Pilih mesin", exact: true })).toBeVisible();
      await use(await context.cookies());
    } finally { await context.close(); }
  }, { scope: "worker" }],
  page: async ({ page, operatorSession }, use) => {
    await page.context().addCookies(operatorSession);
    await use(page);
  },
});

test.skip(process.env.TEST_PUSH_ENABLED !== "1", "Run with TEST_PUSH_ENABLED=1 for the isolated push backend.");
test.use({ reducedMotion: "reduce" });

async function provider(page, permission = "default", outcome = "granted") {
  await page.addInitScript(({ permission, outcome }) => {
    window.pushProbe = { requests: 0, subscriptions: 0, gesture: false };
    Object.defineProperty(window, "Notification", { configurable: true, value: {
      permission,
      requestPermission() {
        window.pushProbe.requests++;
        window.pushProbe.gesture = navigator.userActivation.isActive;
        this.permission = outcome;
        return Promise.resolve(outcome);
      },
    } });
    window.PushManager = class {};
    let subscription = null;
    const worker = { pushManager: {
      async getSubscription() { return subscription; },
      async subscribe(options) {
        window.pushProbe.subscriptions++;
        subscription = {
          endpoint: "https://fcm.googleapis.com/fcm/send/opening-test",
          options,
          async unsubscribe() { subscription = null; return true; },
          toJSON() { return { endpoint: this.endpoint, keys: { p256dh: "a".repeat(87), auth: "b".repeat(22) } }; },
        };
        return subscription;
      },
    }, async getNotifications() { return []; } };
    Object.defineProperty(navigator, "serviceWorker", { configurable: true, value: {
      async register() { return worker; }, async getRegistration() { return worker; }, ready: Promise.resolve(worker),
    } });
  }, { permission, outcome });
}

async function login(page) {
  await page.goto("/");
  // Each case opens the app with a real cookie session in a fresh browser context.
  await expect(page.getByRole("heading", { name: "Pilih mesin", exact: true })).toBeVisible();
}

test("opening prompt waits for gesture, subscribes and stays dismissed during navigation", async ({ page }) => {
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await provider(page);
  await login(page);
  const dialog = page.getByRole("dialog", { name: "Notifikasi pesan baru" });
  await expect(dialog).toBeVisible();
  expect(await page.evaluate(() => window.pushProbe.requests)).toBe(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: test.info().outputPath("opening-prompt.png") });
  expect((await new AxeBuilder({ page }).include("dialog").withTags(["wcag2a", "wcag2aa", "wcag21aa"]).analyze()).violations).toEqual([]);
  await page.evaluate(() => document.documentElement.dataset.theme = "dark");
  expect((await new AxeBuilder({ page }).include("dialog").withTags(["wcag2a", "wcag2aa", "wcag21aa"]).analyze()).violations).toEqual([]);
  await page.screenshot({ path: test.info().outputPath("opening-prompt-dark.png") });
  await page.getByRole("button", { name: "Izinkan notifikasi" }).focus();
  await expect(page.getByRole("button", { name: "Izinkan notifikasi" })).toBeFocused();
  const saved = page.waitForResponse((response) => response.url().endsWith("/push/subscription") && response.request().method() === "PUT");
  await page.getByRole("button", { name: "Izinkan notifikasi" }).click();
  expect((await saved).status()).toBe(204);
  await expect(dialog).toHaveCount(0);
  expect(await page.evaluate(() => window.pushProbe)).toEqual({ requests: 1, subscriptions: 1, gesture: true });
  await page.getByRole("link", { name: /MAILENDER/ }).first().click();
  await expect(dialog).toHaveCount(0);
  expect(await page.evaluate(() => window.pushProbe.requests)).toBe(1);
  expect(errors).toEqual([]);
});

test("Nanti and Escape dismiss without requesting permission", async ({ page }) => {
  await provider(page);
  await login(page);
  await page.getByRole("button", { name: "Nanti", exact: true }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(await page.evaluate(() => window.pushProbe.requests)).toBe(0);
  await page.reload();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(await page.evaluate(() => window.pushProbe.requests)).toBe(0);
});

test("granted permission establishes subscription without prompt", async ({ page }) => {
  await provider(page, "granted");
  const saved = page.waitForResponse((response) => response.url().endsWith("/push/subscription") && response.request().method() === "PUT");
  await login(page);
  expect((await saved).status()).toBe(204);
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(await page.evaluate(() => window.pushProbe.requests)).toBe(0);
  expect(await page.evaluate(() => window.pushProbe.subscriptions)).toBe(1);
});

test("denied permission does not nag", async ({ page }) => {
  await provider(page, "denied");
  await login(page);
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(await page.evaluate(() => window.pushProbe)).toEqual({ requests: 0, subscriptions: 0, gesture: false });
});

test("new denial explains settings without offering another request", async ({ page }) => {
  await provider(page, "default", "denied");
  await login(page);
  await page.getByRole("button", { name: "Izinkan notifikasi" }).click();
  await expect(page.getByText("Notifikasi diblokir.", { exact: false })).toBeVisible();
  await expect(page.getByRole("button", { name: "Izinkan notifikasi" })).toHaveCount(0);
  expect(await page.evaluate(() => window.pushProbe.requests)).toBe(1);
});

test("unsupported browser has no permission prompt", async ({ page }) => {
  await page.addInitScript(() => { delete window.PushManager; });
  await login(page);
  await expect(page.getByRole("dialog")).toHaveCount(0);
});

import { test as base, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

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
    await page.addInitScript(({ session, origin }) => {
      if (location.origin === origin && !sessionStorage.getItem("machine-logbook-session")) {
        sessionStorage.setItem("machine-logbook-session", session);
      }
    }, operatorSession);
    await use(page);
  },
});

async function accessible(page) {
  const results = await new AxeBuilder({ page })
    .withTags(["wcag2a", "wcag2aa", "wcag21aa"])
    .analyze();
  expect(results.violations).toEqual([]);
}

test("no FOUT: blocking script applies data-theme before first render", async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem("theme", "dark");
  });
  await page.goto("/");
  const theme = await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
  expect(theme).toBe("dark");

  const metaThemeColor = await page.evaluate(() =>
    document.querySelector('meta[name="theme-color"]')?.getAttribute("content")
  );
  expect(metaThemeColor).toBe("#12151b");
});

test("login page theme toggle switches theme, persists to localStorage, and satisfies WCAG AA", async ({ browser }, workerInfo) => {
  const context = await browser.newContext({ baseURL: workerInfo.project.use.baseURL });
  const page = await context.newPage();
  try {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: "Masuk ke logbook", exact: true })).toBeVisible();

    // Default light mode
    expect(await page.evaluate(() => document.documentElement.getAttribute("data-theme"))).toBe("light");
    await accessible(page);

    // Toggle to dark
    const toggle = page.getByRole("button", { name: "Ganti ke mode gelap" });
    await expect(toggle).toBeVisible();
    await toggle.click();

    expect(await page.evaluate(() => document.documentElement.getAttribute("data-theme"))).toBe("dark");
    expect(await page.evaluate(() => localStorage.getItem("theme"))).toBe("dark");
    await expect(page.getByRole("button", { name: "Ganti ke mode terang" })).toBeVisible();
    await accessible(page);

    // Reload persists dark mode
    await page.reload();
    expect(await page.evaluate(() => document.documentElement.getAttribute("data-theme"))).toBe("dark");
    await expect(page.getByRole("button", { name: "Ganti ke mode terang" })).toBeVisible();

    // Toggle back to light
    await page.getByRole("button", { name: "Ganti ke mode terang" }).click();
    expect(await page.evaluate(() => document.documentElement.getAttribute("data-theme"))).toBe("light");
    expect(await page.evaluate(() => localStorage.getItem("theme"))).toBe("light");
  } finally {
    await context.close();
  }
});

test("machine list and conversation views comply with WCAG AA in both light and dark themes", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Pilih mesin", exact: true })).toBeVisible();

  // Test layout toggle in machine list (light mode)
  expect(await page.evaluate(() => document.documentElement.getAttribute("data-theme"))).toBe("light");
  await accessible(page);

  // Switch to dark mode via header toggle
  const headerToggle = page.locator(".app-header .theme-toggle");
  await expect(headerToggle).toBeVisible();
  await headerToggle.click();

  expect(await page.evaluate(() => document.documentElement.getAttribute("data-theme"))).toBe("dark");
  await accessible(page);

  // Enter machine conversation
  await page.getByRole("link", { name: "MAILENDER 222", exact: true }).click();
  await expect(page.getByRole("heading", { name: "MAILENDER 222", exact: true })).toBeVisible();

  // Dark mode conversation accessibility
  expect(await page.evaluate(() => document.documentElement.getAttribute("data-theme"))).toBe("dark");
  await accessible(page);

  // Toggle in conversation heading
  const convToggle = page.locator(".conversation-heading .theme-toggle");
  await expect(convToggle).toBeVisible();
  await convToggle.click();

  // Light mode conversation accessibility
  expect(await page.evaluate(() => document.documentElement.getAttribute("data-theme"))).toBe("light");
  await accessible(page);
});

test("system preference listener updates theme when no stored choice", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await page.goto("/");
  expect(await page.evaluate(() => document.documentElement.getAttribute("data-theme"))).toBe("dark");

  await page.emulateMedia({ colorScheme: "light" });
  await expect.poll(async () => {
    return await page.evaluate(() => document.documentElement.getAttribute("data-theme"));
  }).toBe("light");
});

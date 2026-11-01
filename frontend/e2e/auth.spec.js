import { test, expect } from "@playwright/test";

async function login(page) {
  await page.goto("/");
  await page.getByLabel("Nama pengguna", { exact: true }).fill("operator");
  await page.getByLabel("Kata sandi", { exact: true }).fill("frontend-test-only-123");
  await page.getByRole("button", { name: "Masuk", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Pilih mesin", exact: true })).toBeVisible();
}

test("cookie session survives reload and browser restart, logout persists", async ({ browser, baseURL }) => {
  const original = await browser.newContext({ baseURL });
  let restarted;
  try {
    const page = await original.newPage();
    await login(page);
    await page.reload();
    await expect(page.getByRole("heading", { name: "Pilih mesin", exact: true })).toBeVisible();
    const state = await original.storageState();
    const cookie = state.cookies.find((item) => item.name === "logbook_session");
    expect(cookie).toMatchObject({ httpOnly: true, sameSite: "Strict", path: "/api/v1" });
    expect(cookie.expires - Date.now() / 1000).toBeGreaterThan(29 * 86400);
    expect(await page.evaluate(() => sessionStorage.getItem("machine-logbook-session"))).toBeNull();
    expect(state.origins.flatMap((origin) => origin.localStorage).some((item) => item.name === "machine-logbook-session")).toBe(false);
    await original.close();
    restarted = await browser.newContext({ baseURL, storageState: state });
    const restored = await restarted.newPage();
    await restored.goto("/");
    await expect(restored.getByRole("heading", { name: "Pilih mesin", exact: true })).toBeVisible();
    await restored.getByRole("button", { name: "Keluar dari akun" }).click();
    await expect(restored.getByRole("button", { name: "Masuk", exact: true })).toBeVisible();
    await restored.reload();
    await expect(restored.getByRole("button", { name: "Masuk", exact: true })).toBeVisible();
    expect((await restarted.cookies()).some((item) => item.name === "logbook_session")).toBe(false);
  } finally {
    await original.close();
    await restarted?.close();
  }
});

test("failed logout preserves identity until server revocation succeeds", async ({ page }) => {
  await login(page);
  await page.route("**/api/v1/auth/logout", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: { code: "internal_error" } }) }));
  await page.getByRole("button", { name: "Keluar dari akun" }).click();
  await expect(page.getByRole("heading", { name: "Pilih mesin", exact: true })).toBeVisible();
  await expect(page.getByText("Server belum dapat menyelesaikan permintaan. Coba lagi sebentar.")).toBeVisible();
  await page.unroute("**/api/v1/auth/logout");
  await page.reload();
  await expect(page.getByRole("heading", { name: "Pilih mesin", exact: true })).toBeVisible();
});

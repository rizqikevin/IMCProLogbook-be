import { test, expect } from "@playwright/test";

test("production PWA manifest, service worker, offline fallback and private cache boundary", async ({
  page,
  context,
}) => {
  await page.goto("/login");
  const manifestResponse = await page.request.get("/manifest.webmanifest");
  expect(manifestResponse.headers()["content-type"]).toContain(
    "application/manifest+json",
  );
  const workerResponse = await page.request.get("/sw.js");
  expect(workerResponse.headers()["cache-control"]).toBe("no-cache");
  await page.evaluate(() => navigator.serviceWorker.ready);
  await page.reload();
  await expect
    .poll(() => page.evaluate(() => !!navigator.serviceWorker.controller))
    .toBe(true);
  const manifest = await page.evaluate(async () =>
    (await fetch("/manifest.webmanifest")).json(),
  );
  expect(manifest.display).toBe("standalone");
  expect(manifest.start_url).toBe("/");
  for (const icon of manifest.icons) {
    const dimensions = await page.evaluate(async (src) => {
      const image = new Image();
      image.src = src;
      await image.decode();
      return `${image.naturalWidth}x${image.naturalHeight}`;
    }, icon.src);
    expect(dimensions).toBe(icon.sizes);
  }
  const cdp = await context.newCDPSession(page);
  const { installabilityErrors } = await cdp.send(
    "Page.getInstallabilityErrors",
  );
  expect(installabilityErrors).toEqual([]);
  await page.evaluate(() => fetch("/api/v1/auth/me"));
  const cachedURLs = await page.evaluate(async () => {
    const keys = await caches.keys();
    const requests = await Promise.all(
      keys.map(async (key) => (await caches.open(key)).keys()),
    );
    return requests
      .flat()
      .map((request) => new URL(request.url).pathname)
      .sort();
  });
  expect(cachedURLs).toEqual(["/offline.css", "/offline.html"]);
  await context.setOffline(true);
  await page.goto("/logbooks/offline-test");
  await expect(
    page.getByRole("heading", { name: "Koneksi terputus" }),
  ).toBeVisible();
  await context.setOffline(false);
  await page.getByRole("link", { name: "Coba lagi" }).click();
  await expect(page).toHaveURL(/\/login$/);
});

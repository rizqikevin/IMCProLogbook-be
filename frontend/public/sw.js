const CACHE = "logbook-offline-v1";
const OFFLINE = "/offline.html";
const DEVELOPMENT = new URL(self.location.href).searchParams.has("dev");

self.addEventListener("install", (event) => {
  if (DEVELOPMENT) return;
  event.waitUntil(
    caches.open(CACHE).then(async (cache) => {
      for (const resource of [OFFLINE, "/offline.css"]) {
        const response = await fetch(resource, { cache: "reload" });
        // An Access login redirect must never become the offline page.
        if (!response.ok || response.redirected)
          throw new Error("Offline page unavailable");
        await cache.put(resource, response);
      }
    }),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) =>
        Promise.all(
          keys
            .filter(
              (key) => key.startsWith("logbook-offline-") && key !== CACHE,
            )
            .map((key) => caches.delete(key)),
        ),
      ),
  );
});

self.addEventListener("fetch", (event) => {
  if (DEVELOPMENT) return;
  const url = new URL(event.request.url);
  if (
    url.origin === self.location.origin &&
    url.pathname === "/offline.css" &&
    event.request.method === "GET"
  ) {
    event.respondWith(
      caches
        .match("/offline.css")
        .then((response) => response || fetch(event.request)),
    );
    return;
  }
  if (
    event.request.method !== "GET" ||
    url.origin !== self.location.origin ||
    url.pathname.startsWith("/api/") ||
    url.pathname.startsWith("/cdn-cgi/") ||
    event.request.mode !== "navigate"
  )
    return;
  event.respondWith(
    fetch(event.request).catch(
      async () => (await caches.match(OFFLINE)) || Response.error(),
    ),
  );
});

function notificationDestination(value) {
  if (typeof value !== "string") return "/";
  try {
    const url = new URL(value, self.location.origin);
    return url.origin === self.location.origin && /^\/machines\/[A-Za-z0-9_-]+$/.test(url.pathname)
      && !url.search && !url.hash ? url.pathname : "/";
  } catch { return "/"; }
}

self.addEventListener("push", (event) => {
  let payload;
  try { payload = event.data?.json(); } catch { /* Keep malformed payloads generic. */ }
  event.waitUntil(self.registration.showNotification("Pesan baru", {
    body: "Ada pesan baru di percakapan mesin.",
    icon: "/icons/icon-192.png",
    tag: typeof payload?.tag === "string" && /^message-[A-Za-z0-9_-]+$/.test(payload.tag) ? payload.tag : "message",
    data: { url: notificationDestination(payload?.url) },
  }));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const destination = new URL(notificationDestination(event.notification.data?.url), self.location.origin).href;
  event.waitUntil((async () => {
    const windows = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
    for (const client of windows) {
      if (new URL(client.url).origin !== self.location.origin) continue;
      try {
        const navigated = await client.navigate(destination);
        if (navigated) { await navigated.focus(); return; }
      } catch { /* Another app window may have closed meanwhile. */ }
    }
    await self.clients.openWindow(destination);
  })());
});

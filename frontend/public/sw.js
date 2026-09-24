const CACHE = "logbook-offline-v1";
const OFFLINE = "/offline.html";

self.addEventListener("install", (event) => {
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

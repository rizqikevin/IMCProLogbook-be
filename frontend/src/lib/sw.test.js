import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import { beforeEach, expect, it, vi } from "vitest";

const source = readFileSync(new URL("../../public/sw.js", import.meta.url), "utf8");
let handlers;
let worker;

beforeEach(() => {
  handlers = {};
  worker = {
    location: { href: "https://logbook.test/sw.js", origin: "https://logbook.test" },
    addEventListener: (name, handler) => { handlers[name] = handler; },
    registration: { showNotification: vi.fn().mockResolvedValue(undefined) },
    clients: { matchAll: vi.fn().mockResolvedValue([]), openWindow: vi.fn().mockResolvedValue(undefined) },
  };
  runInNewContext(source, { self: worker, URL });
});

async function dispatch(name, event) {
  let pending;
  handlers[name]({ ...event, waitUntil: (value) => { pending = value; } });
  await pending;
}

it("keeps backend machine URLs and message UUID tags without exposing message text", async () => {
  const payload = { url: "/machines/42", tag: "message-123e4567-e89b-12d3-a456-426614174000", body: "private text", title: "private machine" };
  await dispatch("push", { data: { json: () => payload } });
  expect(worker.registration.showNotification).toHaveBeenCalledWith("Pesan baru", expect.objectContaining({
    body: "Ada pesan baru di percakapan mesin.", tag: payload.tag, data: { url: payload.url },
  }));
});

it.each(["https://evil.test/machines/42", "//evil.test/machines/42", "/machines/42?token=secret", "/machines/42#private", "/admin"])("rejects unsafe notification destination %s", async (url) => {
  await dispatch("push", { data: { json: () => ({ url, tag: "invalid tag" }) } });
  expect(worker.registration.showNotification).toHaveBeenCalledWith("Pesan baru", expect.objectContaining({ tag: "message", data: { url: "/" } }));
});

it("shows a generic notification for malformed payloads", async () => {
  await dispatch("push", { data: { json: () => { throw new Error("invalid JSON"); } } });
  expect(worker.registration.showNotification).toHaveBeenCalledWith("Pesan baru", expect.objectContaining({ data: { url: "/" } }));
});

it("navigates and focuses an existing app window on notification click", async () => {
  const focus = vi.fn().mockResolvedValue(undefined);
  const navigate = vi.fn().mockResolvedValue({ focus });
  worker.clients.matchAll.mockResolvedValue([{ url: "https://logbook.test/", navigate }]);
  const close = vi.fn();
  await dispatch("notificationclick", { notification: { close, data: { url: "/machines/42" } } });
  expect(close).toHaveBeenCalledOnce();
  expect(navigate).toHaveBeenCalledWith("https://logbook.test/machines/42");
  expect(focus).toHaveBeenCalledOnce();
  expect(worker.clients.openWindow).not.toHaveBeenCalled();
});

it("opens the machine when an existing window closes during navigation", async () => {
  worker.clients.matchAll.mockResolvedValue([{ url: "https://logbook.test/", navigate: vi.fn().mockRejectedValue(new Error("closed")) }]);
  await dispatch("notificationclick", { notification: { close: vi.fn(), data: { url: "/machines/42" } } });
  expect(worker.clients.openWindow).toHaveBeenCalledWith("https://logbook.test/machines/42");
});

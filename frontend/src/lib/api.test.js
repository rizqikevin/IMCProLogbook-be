import { afterEach, expect, it, vi } from "vitest";
import { advanceSession, connectMessages, photoPath, request, requestPhoto, rotatePhoto } from "./api";

afterEach(() => { advanceSession(); vi.useRealTimers(); vi.unstubAllGlobals(); });

it("sends multipart without JSON encoding or an explicit multipart boundary", async () => {
  const fetch = vi.fn().mockResolvedValue({ ok: true, status: 201, json: async () => ({ id: "message" }) });
  vi.stubGlobal("fetch", fetch);
  const form = new FormData();
  form.append("text", "  line\nline  ");
  await request("/machines/1/messages", { method: "POST", body: form });
  expect(fetch.mock.calls[0][1].body).toBe(form);
  expect(fetch.mock.calls[0][1].headers).toEqual({ Accept: "application/json" });
  expect(fetch.mock.calls[0][1].credentials).toBe("include");
});

it("fetches both historical and message photos with authentication", async () => {
  vi.stubGlobal("window", { location: { origin: "https://logbook.test" } });
  const image = new Blob(["image"]);
  const fetch = vi.fn().mockResolvedValue({ ok: true, status: 200, blob: async () => image });
  vi.stubGlobal("fetch", fetch);
  for (const url of ["/api/v1/logbooks/book/photos/photo", "/api/v1/machines/1/messages/message/photos/photo"]) {
    expect(await requestPhoto(url)).toBe(image);
    expect(fetch.mock.lastCall[0]).toBe(url);
    expect(fetch.mock.lastCall[1].headers.Authorization).toBeUndefined();
    expect(fetch.mock.lastCall[1].credentials).toBe("include");
  }
  expect(() => requestPhoto("https://other.test/api/v1/photos/photo")).toThrow("Alamat foto tidak valid.");
});

it("rotates stored photos using an unversioned path and the current hash", async () => {
  vi.stubGlobal("window", { location: { origin: "https://logbook.test" } });
  const updated = { id: "photo", sha256: "new", url: "/api/v1/logbooks/book/photos/photo?v=new" };
  const fetch = vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => updated });
  vi.stubGlobal("fetch", fetch);
  for (const path of ["/api/v1/logbooks/book/photos/photo", "/api/v1/machines/1/messages/message/photos/photo"]) {
    expect(await rotatePhoto({ url: `${path}?v=old`, sha256: "old" }, -90)).toEqual(updated);
    expect(fetch.mock.lastCall[0]).toBe(`${path}/rotate`);
    expect(JSON.parse(fetch.mock.lastCall[1].body)).toEqual({ degrees: -90, sha256: "old" });
    expect(fetch.mock.lastCall[1].method).toBe("POST");
    expect(fetch.mock.lastCall[1].credentials).toBe("include");
    expect(photoPath(`${path}?v=old`)).toBe(path.slice(7));
  }
  expect(() => photoPath("https://other.test/api/v1/photos/photo")).toThrow("Alamat foto tidak valid.");
});

it("reports stale rotation without retrying writes", async () => {
  vi.stubGlobal("window", { location: { origin: "https://logbook.test" } });
  const fetch = vi.fn().mockResolvedValue({ ok: false, status: 409, json: async () => ({ error: { code: "conflict" } }) });
  vi.stubGlobal("fetch", fetch);
  await expect(rotatePhoto({ url: "/api/v1/logbooks/book/photos/photo?v=old", sha256: "old" }, 90)).rejects.toThrow("Foto sudah berubah");
  expect(fetch).toHaveBeenCalledOnce();
});

it("authenticates by frame, sends over socket, and never retries uncertain sends", async () => {
  vi.useFakeTimers();
  vi.stubGlobal("window", { location: { href: "https://logbook.test/machines/1" } });
  const sockets = [];
  class Socket {
    static OPEN = 1;
    readyState = 1;
    sent = [];
    constructor(url) { this.url = String(url); sockets.push(this); }
    send(value) { this.sent.push(JSON.parse(value)); }
    close() { this.onclose?.({ code: 1000, reason: "" }); }
  }
  vi.stubGlobal("WebSocket", Socket);
  const events = vi.fn();
  const client = connectMessages(1, events, vi.fn());
  const first = sockets[0];
  expect(first.url).toBe("wss://logbook.test/api/v1/machines/1/messages/ws");
  first.onopen();
  expect(first.sent).toEqual([{ type: "auth", token: "" }]);
  first.onmessage({ data: JSON.stringify({ type: "ready" }) });
  const sent = client.send("hello");
  expect(first.sent[1]).toEqual({ type: "send", request_id: expect.any(String), text: "hello" });
  first.onmessage({ data: JSON.stringify({ type: "ack", request_id: first.sent[1].request_id, message: { id: "persisted" } }) });
  await expect(sent).resolves.toEqual({ id: "persisted" });
  const uncertain = client.send("check history");
  const rejection = expect(uncertain).rejects.toThrow("Status kiriman belum pasti");
  first.close();
  await rejection;
  vi.advanceTimersByTime(3000);
  sockets[1].onopen();
  expect(sockets[1].sent).toEqual([{ type: "auth", token: "" }]);
  client.close();
});

it("ignores stale unauthorized responses after a newer login", async () => {
  const dispatchEvent = vi.fn();
  vi.stubGlobal("window", { dispatchEvent });
  let finish;
  vi.stubGlobal("fetch", vi.fn(() => new Promise((resolve) => { finish = resolve; })));
  const oldRequest = request("/machines");
  advanceSession();
  finish({ ok: false, status: 401, json: async () => ({ error: { code: "unauthorized" } }) });
  await expect(oldRequest).rejects.toMatchObject({ status: 401 });
  expect(dispatchEvent).not.toHaveBeenCalled();
});

it("expires current identity on server 401 but not failed login or logout", async () => {
  const dispatchEvent = vi.fn();
  vi.stubGlobal("window", { dispatchEvent });
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 401, json: async () => ({ error: { code: "unauthorized" } }) }));
  await expect(request("/auth/login", { method: "POST", authenticated: false })).rejects.toMatchObject({ status: 401 });
  await expect(request("/auth/logout", { method: "POST", authenticated: false })).rejects.toMatchObject({ status: 401 });
  expect(dispatchEvent).not.toHaveBeenCalled();
  await expect(request("/machines")).rejects.toMatchObject({ status: 401 });
  expect(dispatchEvent).toHaveBeenCalledOnce();
});

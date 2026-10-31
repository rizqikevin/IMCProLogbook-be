import { afterEach, expect, it, vi } from "vitest";
import { connectMessages, request, requestPhoto, saveSession } from "./api";

afterEach(() => { saveSession(null); vi.useRealTimers(); vi.unstubAllGlobals(); });

it("sends multipart without JSON encoding or an explicit multipart boundary", async () => {
  saveSession({ access_token: "test-token", expires_at: "2099-01-01T00:00:00Z" });
  const fetch = vi.fn().mockResolvedValue({ ok: true, status: 201, json: async () => ({ id: "message" }) });
  vi.stubGlobal("fetch", fetch);
  const form = new FormData();
  form.append("text", "  line\nline  ");
  await request("/machines/1/messages", { method: "POST", body: form });
  expect(fetch.mock.calls[0][1].body).toBe(form);
  expect(fetch.mock.calls[0][1].headers).toEqual({ Accept: "application/json", Authorization: "Bearer test-token" });
});

it("fetches both historical and message photos with authentication", async () => {
  vi.stubGlobal("window", { location: { origin: "https://logbook.test" } });
  saveSession({ access_token: "test-token", expires_at: "2099-01-01T00:00:00Z" });
  const image = new Blob(["image"]);
  const fetch = vi.fn().mockResolvedValue({ ok: true, status: 200, blob: async () => image });
  vi.stubGlobal("fetch", fetch);
  for (const url of ["/api/v1/logbooks/book/photos/photo", "/api/v1/machines/1/messages/message/photos/photo"]) {
    expect(await requestPhoto(url)).toBe(image);
    expect(fetch.mock.lastCall[0]).toBe(url);
    expect(fetch.mock.lastCall[1].headers.Authorization).toBe("Bearer test-token");
  }
  expect(() => requestPhoto("https://other.test/api/v1/photos/photo")).toThrow("Alamat foto tidak valid.");
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
  saveSession({ access_token: "secret", expires_at: "2099-01-01" });
  const events = vi.fn();
  const client = connectMessages(1, events, vi.fn());
  const first = sockets[0];
  expect(first.url).toBe("wss://logbook.test/api/v1/machines/1/messages/ws");
  first.onopen();
  expect(first.sent).toEqual([{ type: "auth", token: "secret" }]);
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
  expect(sockets[1].sent).toEqual([{ type: "auth", token: "secret" }]);
  client.close();
});

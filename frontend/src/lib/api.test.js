import { afterEach, expect, it, vi } from "vitest";
import { request, requestPhoto, saveSession } from "./api";

afterEach(() => { saveSession(null); vi.unstubAllGlobals(); });

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

import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { getSessionGeneration, request } from "./api";
import { disablePush, syncPush } from "./push";

vi.mock("./api", () => ({ getSessionGeneration: vi.fn(), request: vi.fn() }));

let worker;
let subscription;
let storage;

beforeEach(() => {
  vi.resetAllMocks();
  storage = new Map();
  vi.stubGlobal("localStorage", {
    getItem: (key) => storage.get(key) ?? null,
    setItem: (key, value) => storage.set(key, value),
    removeItem: (key) => storage.delete(key),
  });
  getSessionGeneration.mockReturnValue(1);
  request.mockImplementation(async (path) => path === "/push/config" ? { enabled: true, public_key: "AQID" } : undefined);
  subscription = {
    endpoint: "https://push.test/current",
    options: { applicationServerKey: new Uint8Array([1, 2, 3]).buffer },
    unsubscribe: vi.fn().mockResolvedValue(true),
    toJSON: () => ({ endpoint: "https://push.test/current" }),
  };
  worker = { pushManager: {
    getSubscription: vi.fn().mockResolvedValue(subscription),
    subscribe: vi.fn().mockResolvedValue(subscription),
  } };
  vi.stubGlobal("navigator", { serviceWorker: {
    register: vi.fn().mockResolvedValue(worker),
    getRegistration: vi.fn().mockResolvedValue(worker),
    ready: Promise.resolve(worker),
  } });
  vi.stubGlobal("Notification", { permission: "granted", requestPermission: vi.fn().mockResolvedValue("granted") });
  storage.set("logbook-push-owner", JSON.stringify({ userId: "1", endpoint: subscription.endpoint }));
});

afterEach(() => vi.unstubAllGlobals());

it("reports disabled despite granted permission when runtime VAPID is absent", async () => {
  request.mockResolvedValue({ enabled: false, public_key: "" });
  await expect(syncPush(1, true)).resolves.toBe("disabled");
  expect(worker.pushManager.subscribe).not.toHaveBeenCalled();
  expect(request).toHaveBeenCalledTimes(1);
});

it("does not mistake granted permission for a subscription", async () => {
  worker.pushManager.getSubscription.mockResolvedValue(null);
  await expect(syncPush(1)).resolves.toBe("off");
  expect(worker.pushManager.subscribe).not.toHaveBeenCalled();
});

it("removes a stale VAPID subscription without silently enabling a new one", async () => {
  subscription.options.applicationServerKey = new Uint8Array([4, 5, 6]).buffer;
  await expect(syncPush(1)).resolves.toBe("off");
  expect(subscription.unsubscribe).toHaveBeenCalledOnce();
  expect(storage.has("logbook-push-owner")).toBe(false);
  expect(worker.pushManager.subscribe).not.toHaveBeenCalled();
  expect(request).toHaveBeenCalledTimes(1);
});

it("replaces a stale VAPID subscription on explicit enable", async () => {
  subscription.options.applicationServerKey = new Uint8Array([4, 5, 6]).buffer;
  await expect(syncPush(1, true)).resolves.toBe("on");
  expect(subscription.unsubscribe).toHaveBeenCalledOnce();
  expect(worker.pushManager.subscribe).toHaveBeenCalledWith({ userVisibleOnly: true, applicationServerKey: new Uint8Array([1, 2, 3]) });
  expect(request).toHaveBeenLastCalledWith("/push/subscription", { method: "PUT", body: subscription.toJSON() });
});

it("preserves a matching subscription and registers it with the backend", async () => {
  await expect(syncPush(1)).resolves.toBe("on");
  expect(subscription.unsubscribe).not.toHaveBeenCalled();
  expect(worker.pushManager.subscribe).not.toHaveBeenCalled();
  expect(request).toHaveBeenLastCalledWith("/push/subscription", { method: "PUT", body: subscription.toJSON() });
});

it("refuses replacement when browser unsubscribe fails", async () => {
  subscription.options.applicationServerKey = new Uint8Array([4]).buffer;
  subscription.unsubscribe.mockResolvedValue(false);
  await expect(syncPush(1, true)).rejects.toThrow("belum dapat dilepas");
  expect(worker.pushManager.subscribe).not.toHaveBeenCalled();
});

it("removes another account's subscription even when backend push is disabled", async () => {
  request.mockResolvedValue({ enabled: false, public_key: "" });
  storage.set("logbook-push-owner", JSON.stringify({ userId: "2", endpoint: subscription.endpoint }));
  await expect(syncPush(1)).resolves.toBe("disabled");
  expect(subscription.unsubscribe).toHaveBeenCalledOnce();
  expect(storage.has("logbook-push-owner")).toBe(false);
});

it("cleans up a new subscription if the session changes during subscribe", async () => {
  worker.pushManager.getSubscription.mockResolvedValue(null);
  worker.pushManager.subscribe.mockImplementation(async () => {
    getSessionGeneration.mockReturnValue(2);
    return subscription;
  });
  await expect(syncPush(1, true)).rejects.toThrow("Sesi berubah");
  expect(subscription.unsubscribe).toHaveBeenCalledOnce();
  expect(request).toHaveBeenCalledTimes(1);
});

it("does not leave ownership behind after backend registration fails", async () => {
  request.mockImplementation(async (path) => {
    if (path === "/push/config") return { enabled: true, public_key: "AQID" };
    throw new Error("offline");
  });
  await expect(syncPush(1, true)).rejects.toThrow("offline");
  expect(subscription.unsubscribe).toHaveBeenCalledOnce();
  expect(storage.has("logbook-push-owner")).toBe(false);
});

it("preserves browser subscription when logout cleanup fails", async () => {
  vi.stubGlobal("window", { isSecureContext: true, PushManager: {}, Notification: {} });
  request.mockRejectedValue(new Error("offline"));
  await expect(disablePush()).rejects.toThrow("offline");
  expect(request).toHaveBeenCalledWith("/push/subscription", {
    method: "DELETE", body: { endpoint: subscription.endpoint }, authenticated: false,
  });
  expect(subscription.unsubscribe).not.toHaveBeenCalled();
  expect(storage.has("logbook-push-owner")).toBe(true);
});

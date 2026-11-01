import { getSessionGeneration, request } from "./api";

const ownerKey = "logbook-push-owner";
let pending = Promise.resolve();

export function pushSupported() {
  return window.isSecureContext && "serviceWorker" in navigator && "PushManager" in window && "Notification" in window;
}

export function vapidKey(value) {
  const raw = atob(value.replace(/-/g, "+").replace(/_/g, "/").padEnd(Math.ceil(value.length / 4) * 4, "="));
  return Uint8Array.from(raw, (character) => character.charCodeAt(0));
}

function owner() {
  try { return JSON.parse(localStorage.getItem(ownerKey)); } catch { return null; }
}

function remember(userId, endpoint) {
  // Without durable ownership, never silently restore a shared browser subscription.
  localStorage.setItem(ownerKey, JSON.stringify({ userId: String(userId), endpoint }));
}

function serialized(action) {
  const result = pending.then(action);
  pending = result.catch(() => {});
  return result;
}

async function registration() {
  await navigator.serviceWorker.register(import.meta.env.DEV ? "/sw.js?dev=1" : "/sw.js", { updateViaCache: "none" });
  return navigator.serviceWorker.ready;
}

export function syncPush(userId, enable = false) {
  const generation = getSessionGeneration();
  // Permission must start synchronously inside the user's click, before network work.
  const permission = enable && Notification.permission === "default" ? Notification.requestPermission() : Promise.resolve(Notification.permission);
  return serialized(async () => {
    const allowed = await permission;
    const current = () => {
      if (getSessionGeneration() !== generation) throw new Error("Sesi berubah. Muat ulang halaman sebelum mengatur notifikasi.");
    };
    current();
    const config = await request("/push/config");
    current();
    const enabled = config.enabled && config.public_key;
    const worker = enabled ? await registration() : await navigator.serviceWorker.getRegistration();
    current();
    let subscription = await worker?.pushManager.getSubscription();
    const stored = owner();
    const key = enabled ? vapidKey(config.public_key) : null;
    const subscribedKey = subscription?.options?.applicationServerKey;
    const bytes = subscribedKey ? new Uint8Array(subscribedKey) : null;
    const staleKey = key && (!bytes || bytes.length !== key.length || key.some((value, index) => value !== bytes[index]));
    if (subscription && (stored?.userId !== String(userId) || stored?.endpoint !== subscription.endpoint || staleKey)) {
      if (!await subscription.unsubscribe()) throw new Error("Langganan akun sebelumnya belum dapat dilepas. Coba lagi.");
      localStorage.removeItem(ownerKey);
      subscription = null;
    }
    if (!enabled) return "disabled";
    if (allowed !== "granted") return allowed === "denied" ? "denied" : "off";
    if (!subscription && !enable) return "off";
    if (!subscription) subscription = await worker.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: key });
    try {
      current();
      remember(userId, subscription.endpoint);
      await request("/push/subscription", { method: "PUT", body: subscription.toJSON() });
      current();
    } catch (error) {
      if (await subscription.unsubscribe()) localStorage.removeItem(ownerKey);
      throw error;
    }
    return "on";
  });
}

export function disablePush() {
  const generation = getSessionGeneration();
  return serialized(async () => {
    if (!pushSupported()) return;
    const worker = await navigator.serviceWorker.getRegistration();
    const subscription = await worker?.pushManager.getSubscription();
    if (generation !== getSessionGeneration()) throw new Error("Sesi berubah. Muat ulang halaman sebelum mengatur notifikasi.");
    if (subscription) {
      await request("/push/subscription", { method: "DELETE", body: { endpoint: subscription.endpoint }, authenticated: false });
      if (generation !== getSessionGeneration()) throw new Error("Sesi berubah. Muat ulang halaman sebelum mengatur notifikasi.");
      if (!await subscription.unsubscribe()) throw new Error("Notifikasi belum dapat dimatikan di browser. Coba lagi sebelum keluar.");
    }
    localStorage.removeItem(ownerKey);
    for (const notification of await worker?.getNotifications() || []) notification.close();
  });
}

const base = (import.meta.env.VITE_API_BASE_URL || "").replace(/\/$/, "");
const sessionKey = "machine-logbook-session";
let memorySession = null;
let socketRequestSequence = 0;

export function readSession() {
  try {
    const stored = JSON.parse(sessionStorage.getItem(sessionKey));
    return stored?.access_token && stored?.expires_at ? stored : memorySession;
  } catch {
    return memorySession;
  }
}

export function saveSession(session) {
  memorySession = session;
  try {
    if (session) sessionStorage.setItem(sessionKey, JSON.stringify(session));
    else sessionStorage.removeItem(sessionKey);
  } catch {
    /* Session remains in memory when browser storage is unavailable. */
  }
}

export class ApiError extends Error {
  constructor(message, status = 0, code = "network_error", requestId = "") {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }
}

function responseError(status, payload) {
  const messages = {
    unauthorized:
      "Nama pengguna atau kata sandi salah, atau sesi Anda sudah berakhir.",
    forbidden: "Akun Anda tidak memiliki izin untuk tindakan ini.",
    not_found: "Arsip atau foto ini tidak ditemukan. Mungkin sudah dihapus.",
    conflict:
      "Arsip untuk mesin, tanggal, dan shift ini sudah ada. Buka arsip tersebut untuk menambahkan halaman.",
    photo_limit: "Jumlah halaman arsip sudah mencapai batas.",
    payload_too_large:
      "Ukuran unggahan terlalu besar. Kurangi jumlah atau ukuran foto.",
    rate_limited:
      "Terlalu banyak percobaan masuk. Tunggu satu menit lalu coba lagi.",
    upload_busy:
      "Server sedang menerima unggahan lain. Tunggu sebentar lalu coba lagi.",
    timeout: "Waktu permintaan habis. Periksa arsip sebelum mengunggah ulang.",
    origin_not_allowed:
      "Alamat aplikasi belum diizinkan oleh server. Hubungi administrator.",
    upload_expired:
      "Waktu unggahan berakhir. Periksa arsip sebelum mencoba lagi.",
    internal_error:
      "Server belum dapat menyelesaikan permintaan. Coba lagi sebentar.",
  };
  const error = payload?.error || {};
  return new ApiError(
    messages[error.code] ||
      error.message ||
      "Server tidak dapat dihubungi. Periksa koneksi Anda.",
    status,
    error.code || "server_error",
    error.request_id,
  );
}

function expireSession(token) {
  if (token && readSession()?.access_token === token) {
    saveSession(null);
    window.dispatchEvent(new Event("logbook:session-expired"));
  }
}

export async function request(
  path,
  { method = "GET", body, signal, authenticated = true, blob = false } = {},
) {
  const token = authenticated ? readSession()?.access_token : null;
  const headers = { Accept: blob ? "image/*" : "application/json" };
  if (token) headers.Authorization = `Bearer ${token}`;
  const multipart = body instanceof FormData;
  if (body !== undefined && !multipart) headers["Content-Type"] = "application/json";
  let response;
  try {
    response = await fetch(`${base}/api/v1${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : multipart ? body : JSON.stringify(body),
      signal,
      credentials: "omit",
      cache: "no-store",
    });
  } catch (error) {
    if (error.name === "AbortError") throw error;
    throw new ApiError("Koneksi terputus. Periksa jaringan lalu coba lagi.");
  }
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    if (response.status === 401 && authenticated) expireSession(token);
    throw responseError(response.status, payload);
  }
  if (response.status === 204) return null;
  if (blob) return response.blob();
  try {
    return await response.json();
  } catch {
    throw new ApiError(
      "Respons server tidak valid. Periksa konfigurasi alamat API.",
      response.status,
    );
  }
}

export function queryString(filters) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(filters)) {
    if (value !== "" && value !== null && value !== undefined)
      query.set(key, String(value));
  }
  return query.toString();
}

export function requestPhoto(url, signal) {
  const api = new URL(`${base || window.location.origin}/api/v1/`, window.location.origin);
  const target = new URL(url, api);
  if (target.origin !== api.origin || !target.pathname.startsWith("/api/v1/"))
    throw new ApiError("Alamat foto tidak valid.");
  return request(`${target.pathname.slice(7)}${target.search}`, { blob: true, signal });
}

export function connectMessages(machineId, onEvent, onStatus) {
  const url = new URL(`${base}/api/v1/machines/${encodeURIComponent(machineId)}/messages/ws`, window.location.href);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  let socket, retry, stopped = false, ready = false;
  const pending = new Map();
  const uncertain = () => new ApiError("Koneksi terputus. Status kiriman belum pasti. Periksa riwayat sebelum mengirim ulang.");
  function open() {
    onStatus("connecting");
    const token = readSession()?.access_token;
    socket = new WebSocket(url);
    const current = socket;
    const deadline = setTimeout(() => current.close(), 10000);
    current.onopen = () => current.send(JSON.stringify({ type: "auth", token }));
    current.onmessage = ({ data }) => {
      let event;
      try { event = JSON.parse(data); } catch { current.close(); return; }
      if (event.type === "ready") {
        clearTimeout(deadline);
        ready = true;
        onStatus("connected");
        onEvent(event);
      } else if (event.type === "ack" || event.type === "error") {
        const waiting = pending.get(event.request_id);
        if (!waiting) return;
        clearTimeout(waiting.timer);
        pending.delete(event.request_id);
        if (event.type === "ack") waiting.resolve(event.message);
        else waiting.reject(responseError(400, event));
      } else if (ready) onEvent(event);
    };
    current.onerror = () => current.close();
    current.onclose = (event) => {
      clearTimeout(deadline);
      ready = false;
      for (const waiting of pending.values()) {
        clearTimeout(waiting.timer);
        waiting.reject(uncertain());
      }
      pending.clear();
      if (stopped) return;
      onStatus("disconnected");
      if (event.code === 1008 && event.reason === "unauthorized") { expireSession(token); return; }
      retry = setTimeout(open, 3000);
    };
  }
  open();
  return {
    send(text) {
      if (!ready || socket.readyState !== WebSocket.OPEN) return Promise.reject(new ApiError("Koneksi realtime belum siap. Tunggu sampai tersambung."));
      const requestId = String(++socketRequestSequence);
      return new Promise((resolve, reject) => {
        const timer = setTimeout(() => { pending.delete(requestId); reject(uncertain()); socket.close(); }, 30000);
        pending.set(requestId, { resolve, reject, timer });
        try { socket.send(JSON.stringify({ type: "send", request_id: requestId, text })); }
        catch { clearTimeout(timer); pending.delete(requestId); reject(uncertain()); socket.close(); }
      });
    },
    close() { stopped = true; clearTimeout(retry); socket.close(); },
  };
}


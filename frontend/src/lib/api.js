const base = (import.meta.env.VITE_API_BASE_URL || "").replace(/\/$/, "");
const sessionKey = "machine-logbook-session";
let memorySession = null;

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
  if (body !== undefined) headers["Content-Type"] = "application/json";
  let response;
  try {
    response = await fetch(`${base}/api/v1${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
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

export function uploadLogbook({
  bookId,
  machineId,
  date,
  shiftId,
  files,
  signal,
  onProgress,
}) {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    if (!bookId) {
      form.append("machine_id", machineId);
      form.append("log_date", date);
      form.append("shift_id", shiftId);
    }
    files.forEach((file) => form.append("photos", file, file.name));
    const xhr = new XMLHttpRequest();
    const token = readSession()?.access_token;
    xhr.open(
      "POST",
      `${base}/api/v1/logbooks${bookId ? `/${encodeURIComponent(bookId)}/photos` : ""}`,
    );
    if (token) xhr.setRequestHeader("Authorization", `Bearer ${token}`);
    xhr.setRequestHeader("Accept", "application/json");
    xhr.timeout = 130_000;
    const abort = () => xhr.abort();
    const cleanup = () => signal?.removeEventListener("abort", abort);
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable)
        onProgress?.(Math.round((event.loaded / event.total) * 100));
    };
    xhr.onload = () => {
      cleanup();
      let payload;
      try {
        payload = JSON.parse(xhr.responseText);
      } catch {
        payload = null;
      }
      if (xhr.status >= 200 && xhr.status < 300 && payload?.id)
        resolve(payload);
      else {
        if (xhr.status === 401) expireSession(token);
        reject(responseError(xhr.status, payload));
      }
    };
    xhr.onerror = () => {
      cleanup();
      reject(
        new ApiError(
          "Koneksi terputus saat mengunggah. Periksa arsip sebelum mencoba lagi.",
        ),
      );
    };
    xhr.ontimeout = () => {
      cleanup();
      reject(
        new ApiError(
          "Waktu unggah habis. Periksa arsip sebelum mencoba lagi.",
          0,
          "timeout",
        ),
      );
    };
    xhr.onabort = () => {
      cleanup();
      reject(new DOMException("Upload canceled", "AbortError"));
    };
    if (signal?.aborted) {
      reject(new DOMException("Upload canceled", "AbortError"));
      return;
    }
    signal?.addEventListener("abort", abort, { once: true });
    xhr.send(form);
  });
}

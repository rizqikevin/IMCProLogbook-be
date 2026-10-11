export function formatDate(value, options = {}) {
  return new Intl.DateTimeFormat("id-ID", {
    day: "numeric",
    month: "long",
    year: "numeric",
    ...options,
  }).format(new Date(`${value}T12:00:00`));
}

export function formatTime(value) {
  return new Intl.DateTimeFormat("id-ID", {
    day: "numeric",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

export function formatBytes(bytes) {
  if (bytes < 1024 * 1024)
    return `${Math.ceil(bytes / 1024).toLocaleString("id-ID")} KB`;
  return `${(bytes / (1024 * 1024)).toLocaleString("id-ID", { maximumFractionDigits: 1 })} MB`;
}

export const limits = {
  photos: 20,
  photoBytes: 10 * 1024 * 1024,
  requestBytes: 95_000_000,
};

export function validateSelection(existing, incoming) {
  if (existing.length + incoming.length > limits.photos) {
    return `Maksimal ${limits.photos} foto untuk unggahan ini.`;
  }
  for (const file of incoming) {
    if (!["image/jpeg", "image/png", "image/webp"].includes(file.type))
      return "Gunakan foto JPG, PNG, atau WebP. Ubah foto HEIC menjadi JPG terlebih dahulu.";
    if (!file.size || file.size > limits.photoBytes)
      return `Foto “${file.name}” harus berukuran maksimal 10 MB dan tidak boleh kosong.`;
  }
  if (
    [...existing.map((item) => item.file), ...incoming].reduce(
      (sum, file) => sum + file.size,
      0,
    ) > limits.requestBytes
  ) {
    return "Total foto terlalu besar. Gunakan kurang dari 95 MB per unggahan.";
  }
  return "";
}

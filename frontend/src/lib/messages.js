export const textLimit = 20_000;

export function validateMessage(text, photos) {
  if (Array.from(text).length > textLimit)
    return "Pesan maksimal 20.000 karakter.";
  if (!text.trim() && photos.length === 0)
    return "Tulis pesan atau pilih setidaknya satu foto.";
  return "";
}

export function messageForm(text, photos) {
  const form = new FormData();
  if (text !== "") form.append("text", text);
  for (const photo of photos) form.append("photos", photo.file, photo.file.name);
  return form;
}

export function mergeMessages(older, newer) {
  const messages = new Map();
  for (const message of [...older, ...newer]) messages.set(message.id, message);
  return [...messages.values()];
}

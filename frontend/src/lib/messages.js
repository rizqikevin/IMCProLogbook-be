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

export function flattenPhotos(items) {
  const list = [];
  for (const message of items) {
    if (message.photos && message.photos.length > 0) {
      for (const photo of message.photos) {
        list.push({
          ...photo,
          messageId: message.id,
          messageAuthor: message.author?.name || "Operator",
          messageTime: message.created_at,
          messageText: message.text,
          totalInMessage: message.photos.length,
        });
      }
    }
  }
  return list;
}

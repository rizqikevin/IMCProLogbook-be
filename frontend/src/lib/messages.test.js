import { describe, expect, it } from "vitest";
import { flattenPhotos, mergeMessages, messageForm, normalizeMessageText, validateMessage } from "./messages";

describe("message content", () => {
  it("accepts text, photos, or both but rejects blank content", () => {
    expect(validateMessage(" \n\t", [])).not.toBe("");
    expect(validateMessage("catatan", [])).toBe("");
    expect(validateMessage("", [{}])).toBe("");
    expect(validateMessage("catatan", [{}])).toBe("");
  });
  it("counts Unicode code points rather than UTF-16 units", () => {
    expect(validateMessage("𠮷".repeat(20_000), [])).toBe("");
    expect(validateMessage("𠮷".repeat(20_001), [])).not.toBe("");
  });
  it("normalizes only CRLF and validates edited text against existing photos", () => {
    expect(normalizeMessageText("  awal\r\nakhir\r  ")).toBe("  awal\nakhir\r  ");
    expect(validateMessage("a\r\n".repeat(10_000), [])).toBe("");
    expect(validateMessage("\0", [{}])).not.toBe("");
    expect(validateMessage(" \n", [])).not.toBe("");
    expect(validateMessage("", [{ id: "saved-photo" }])).toBe("");
  });
  it("preserves text exactly and includes repeated photos without date or shift", () => {
    const text = "  awal\n\n<script>catatan</script>\takhir  ";
    const file = new File(["photo"], "photo.png", { type: "image/png" });
    const form = messageForm(text, [{ file }, { file }]);
    expect(form.get("text")).toBe(text);
    expect(form.getAll("photos")).toHaveLength(2);
    expect([...form.keys()]).toEqual(["text", "photos", "photos"]);
    expect([...messageForm("", [{ file }]).keys()]).toEqual(["photos"]);
  });
});

describe("timeline pages", () => {
  it("deduplicates overlap while retaining server chronological and stable id order", () => {
    const a = { id: "a", created_at: "2026-01-01T00:00:00Z" };
    const b = { id: "b", created_at: a.created_at };
    const c = { id: "c", created_at: "2026-01-02T00:00:00Z" };
    expect(mergeMessages([a, b], [b, c])).toEqual([a, b, c]);
  });
  it("replaces saved edits without changing timeline order or photo metadata", () => {
    const original = { id: "a", text: "before", created_at: "2026-01-01T00:00:00Z", photos: [{ id: "photo" }], edited_at: null };
    const next = { id: "b", text: "later" };
    const saved = { ...original, text: "after", edited_at: "2026-01-02T00:00:00Z" };
    expect(mergeMessages([original, next], [saved])).toEqual([saved, next]);
    expect(saved.photos).toBe(original.photos);
    expect(saved.created_at).toBe(original.created_at);
  });
});

describe("flattenPhotos", () => {
  it("extracts photos in sequential message order with metadata", () => {
    const m1 = {
      id: "m1",
      created_at: "2026-01-01T08:00:00Z",
      author: { name: "Operator A" },
      text: "Catatan satu",
      photos: [
        { id: "p1", page_number: 1, url: "/photo1" },
        { id: "p2", page_number: 2, url: "/photo2" },
      ],
    };
    const m2 = {
      id: "m2",
      created_at: "2026-01-01T09:00:00Z",
      author: { name: "Operator B" },
      text: "Hanya teks",
      photos: [],
    };
    const m3 = {
      id: "m3",
      created_at: "2026-01-01T10:00:00Z",
      author: null,
      text: "",
      photos: [{ id: "p3", page_number: 1, url: "/photo3" }],
    };

    const photos = flattenPhotos([m1, m2, m3]);
    expect(photos).toHaveLength(3);
    expect(photos[0]).toMatchObject({
      id: "p1",
      messageId: "m1",
      messageAuthor: "Operator A",
      messageText: "Catatan satu",
      totalInMessage: 2,
    });
    expect(photos[1]).toMatchObject({
      id: "p2",
      messageId: "m1",
      messageAuthor: "Operator A",
      messageText: "Catatan satu",
      totalInMessage: 2,
    });
    expect(photos[2]).toMatchObject({
      id: "p3",
      messageId: "m3",
      messageAuthor: "Operator",
      messageText: "",
      totalInMessage: 1,
    });
  });

  it("returns empty array when no messages have photos", () => {
    expect(flattenPhotos([])).toEqual([]);
    expect(flattenPhotos([{ id: "m1", photos: [] }])).toEqual([]);
  });
});

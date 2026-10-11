import { describe, expect, it } from "vitest";
import { mergeMessages, messageForm, validateMessage } from "./messages";

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
});

import { describe, expect, it } from "vitest";
import { formatDate, limits, today, validateSelection } from "./format";
import { queryString } from "./api";

describe("photo selection", () => {
  const photo = (size = 100, type = "image/jpeg") => ({
    name: "page.jpg",
    type,
    size,
  });
  it("bounds a batch and remaining archive pages", () => {
    expect(
      validateSelection(
        [],
        Array.from({ length: 21 }, () => photo()),
      ),
    ).toContain("20");
    expect(validateSelection([{ file: photo() }], [photo()], 1)).toContain("1");
  });
  it("rejects unsupported, empty, oversized, and collectively oversized files", () => {
    expect(validateSelection([], [photo(100, "image/heic")])).toContain("HEIC");
    expect(validateSelection([], [photo(0)])).toContain("kosong");
    expect(validateSelection([], [photo(limits.photoBytes + 1)])).toContain(
      "10 MB",
    );
    expect(
      validateSelection(
        [],
        Array.from({ length: 10 }, () => photo(limits.photoBytes)),
      ),
    ).toContain("95 MB");
  });
  it("allows ordered valid image batches", () => {
    expect(
      validateSelection(
        [{ file: photo() }],
        [photo(100, "image/png"), photo(100, "image/webp")],
      ),
    ).toBe("");
  });
});

it("keeps empty filters out of backend queries", () => {
  expect(
    queryString({ machine_id: "", shift_id: 2, cursor: null, limit: 20 }),
  ).toBe("shift_id=2&limit=20");
});

it("formats business dates without converting them to UTC", () => {
  expect(formatDate("2026-09-23")).toBe("23 September 2026");
  expect(today()).toMatch(/^\d{4}-\d{2}-\d{2}$/);
});

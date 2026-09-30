import { describe, it, expect } from "vitest";
import { adjacentShift, validDate } from "./shifts";
describe("shift chronology", () => {
  it("advances and reverses across dates without skipping shifts", () => {
    expect(adjacentShift("2026-12-31", 3, 1)).toEqual({
      date: "2027-01-01",
      shift: 1,
    });
    expect(adjacentShift("2027-01-01", 1, -1)).toEqual({
      date: "2026-12-31",
      shift: 3,
    });
    expect(adjacentShift("2028-03-01", 1, -1)).toEqual({
      date: "2028-02-29",
      shift: 3,
    });
    expect(adjacentShift("2026-09-30", 1, 1)).toEqual({
      date: "2026-09-30",
      shift: 2,
    });
  });
  it("rejects invalid dates and shifts", () => {
    expect(validDate("2026-02-30")).toBe(false);
    expect(adjacentShift("bad", 1, 1)).toBeNull();
    expect(adjacentShift("2026-09-30", 4, 1)).toBeNull();
  });
});

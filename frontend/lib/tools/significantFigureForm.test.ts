import { describe, expect, it } from "vitest";
import { formatRoundedValue } from "./significantFigureForm";

describe("formatRoundedValue", () => {
  it.each([
    ["9.8123", "0.034", "9.81 ± 0.03"],
    ["-9.8123", "0.034", "-9.81 ± 0.03"],
    ["1234.5", "52.3", "1230 ± 50"],
    // No uncertainty to round to: the rounder's fixed default, and "0.0"
    // for the ± term.
    ["9.8123", "0", "9.8123 ± 0.0"],
  ])("%s ± %s -> %s", (value, uncertainty, want) => {
    expect(formatRoundedValue(value, uncertainty)).toBe(want);
  });

  it.each([
    ["value empty", "", "0.034"],
    ["uncertainty empty", "9.8", ""],
    ["value not a number", "abc", "0.034"],
    ["uncertainty not a number", "9.8", "abc"],
    ["value infinite", "1e999", "0.034"],
  ])("is null when %s", (_, value, uncertainty) => {
    expect(formatRoundedValue(value, uncertainty)).toBeNull();
  });
});

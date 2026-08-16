import { describe, expect, it } from "vitest";
import { convertUnit } from "./unitConversion";

describe("convertUnit", () => {
  it("converts within the length category", () => {
    expect(convertUnit("length", "km", "m", 1)).toBeCloseTo(1000, 10);
    expect(convertUnit("length", "cm", "m", 100)).toBeCloseTo(1, 10);
    expect(convertUnit("length", "in", "cm", 1)).toBeCloseTo(2.54, 10);
  });

  it("converts within the mass category", () => {
    expect(convertUnit("mass", "g", "kg", 500)).toBeCloseTo(0.5, 10);
    expect(convertUnit("mass", "lb", "kg", 1)).toBeCloseTo(0.45359237, 10);
  });

  it("converts within the time category", () => {
    expect(convertUnit("time", "h", "s", 1)).toBeCloseTo(3600, 10);
    expect(convertUnit("time", "min", "s", 2)).toBeCloseTo(120, 10);
  });

  it("converts within the angle category", () => {
    expect(convertUnit("angle", "deg", "rad", 180)).toBeCloseTo(Math.PI, 10);
  });

  it("converts temperature using affine (offset) formulas, not pure scaling", () => {
    expect(convertUnit("temperature", "C", "K", 0)).toBeCloseTo(273.15, 10);
    expect(convertUnit("temperature", "C", "F", 0)).toBeCloseTo(32, 10);
    expect(convertUnit("temperature", "F", "C", 212)).toBeCloseTo(100, 10);
  });

  it("returns the same value converting a unit to itself", () => {
    expect(convertUnit("length", "m", "m", 42)).toBeCloseTo(42, 10);
  });

  it("returns null for an unknown category or unit id", () => {
    expect(convertUnit("bogus", "m", "cm", 1)).toBeNull();
    expect(convertUnit("length", "bogus", "cm", 1)).toBeNull();
    expect(convertUnit("length", "m", "bogus", 1)).toBeNull();
  });
});

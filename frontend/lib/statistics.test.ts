import { describe, expect, it } from "vitest";
import { computeColumnStats } from "./statistics";

describe("computeColumnStats", () => {
  it("computes mean, sample stdev, and sem for a typical sample", () => {
    const { n, mean, stdev, sem } = computeColumnStats([
      2, 4, 4, 4, 5, 5, 7, 9,
    ]);
    expect(n).toBe(8);
    expect(mean).toBeCloseTo(5, 10);
    expect(stdev).toBeCloseTo(2.13809, 4);
    expect(sem).toBeCloseTo(0.75593, 4);
  });

  it("returns null stdev/sem for a single value (n < 2)", () => {
    const { n, mean, stdev, sem } = computeColumnStats([3.5]);
    expect(n).toBe(1);
    expect(mean).toBe(3.5);
    expect(stdev).toBeNull();
    expect(sem).toBeNull();
  });

  it("returns null stdev/sem for an empty array", () => {
    const { n, stdev, sem } = computeColumnStats([]);
    expect(n).toBe(0);
    expect(stdev).toBeNull();
    expect(sem).toBeNull();
  });

  it("computes a zero stdev/sem for identical values", () => {
    const { stdev, sem } = computeColumnStats([5, 5, 5]);
    expect(stdev).toBe(0);
    expect(sem).toBe(0);
  });
});

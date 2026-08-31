import { describe, expect, it } from "vitest";
import { linearRange, logRange, logTicks, ticksForLogRange } from "./logScale";

describe("linearRange", () => {
  it("pads the data range by 5% on each side", () => {
    expect(linearRange([0, 100])).toEqual([-5, 105]);
  });

  it("falls back to +/-1 when every value is identical", () => {
    // 5% of zero spread is zero, which would collapse the axis.
    expect(linearRange([7, 7, 7])).toEqual([6, 8]);
  });

  it("falls back to +/-1 for a single point", () => {
    expect(linearRange([7])).toEqual([6, 8]);
  });

  it("handles negative data", () => {
    expect(linearRange([-100, 0])).toEqual([-105, 5]);
  });

  it("returns undefined when there is no data", () => {
    expect(linearRange([])).toBeUndefined();
  });
});

describe("logRange", () => {
  it("returns log10 bounds padded by 5%", () => {
    // log10(1)=0, log10(100)=2, pad = 0.1
    expect(logRange([1, 100])).toEqual([-0.1, 2.1]);
  });

  it("drops non-positive values, which a log axis cannot show", () => {
    expect(logRange([-5, 0, 1, 100])).toEqual([-0.1, 2.1]);
  });

  it("falls back to +/-0.5 decades when every value is identical", () => {
    const [lo, hi] = logRange([10, 10])!;
    expect(lo).toBeCloseTo(0.5, 10);
    expect(hi).toBeCloseTo(1.5, 10);
  });

  it.each([
    ["empty", []],
    ["all zero", [0, 0]],
    ["all negative", [-1, -2]],
  ])("returns undefined when the data is %s", (_label, values) => {
    expect(logRange(values)).toBeUndefined();
  });
});

describe("logTicks", () => {
  it("places ticks on the 1-2-5 series within a single decade", () => {
    expect(logTicks(1, 9).tickvals).toEqual([1, 2, 5]);
  });

  it("repeats the series across every decade in range", () => {
    expect(logTicks(1, 1000).tickvals).toEqual([
      1, 2, 5, 10, 20, 50, 100, 200, 500, 1000,
    ]);
  });

  it("labels every tick with its full value, dropping no digits", () => {
    // Plotly's own formatting would render 500 as "5" and 20 as "2".
    const { tickvals, ticktext } = logTicks(2, 1000);
    const labels = Object.fromEntries(tickvals.map((v, i) => [v, ticktext[i]]));
    expect(labels[20]).toBe("20");
    expect(labels[500]).toBe("500");
    expect(labels[1000]).toBe("1000");
  });

  it("excludes ticks outside the bounds", () => {
    expect(logTicks(3, 40).tickvals).toEqual([5, 10, 20]);
  });

  it("labels sub-decade ticks without floating-point noise", () => {
    expect(logTicks(0.01, 1).ticktext).toEqual([
      "0.01",
      "0.02",
      "0.05",
      "0.1",
      "0.2",
      "0.5",
      "1",
    ]);
  });

  it.each([
    ["min is zero", 0, 100],
    ["min is negative", -5, 100],
    ["bounds are inverted", 100, 1],
    ["bounds are equal", 10, 10],
  ])("returns empty arrays when %s", (_label, min, max) => {
    expect(logTicks(min, max)).toEqual({ tickvals: [], ticktext: [] });
  });
});

describe("ticksForLogRange", () => {
  it("converts log10 bounds back to real values before picking ticks", () => {
    // [0, 2] in log10 units means 1 to 100.
    expect(ticksForLogRange([0, 2])!.tickvals).toEqual([
      1, 2, 5, 10, 20, 50, 100,
    ]);
  });

  it("returns undefined when there is no range", () => {
    expect(ticksForLogRange(undefined)).toBeUndefined();
  });
});

import { describe, expect, it } from "vitest";
import { parsePastedText } from "@/lib/pasteDataParsing";
import { formatMean, splitColumns, summarizeColumn } from "./measurementStats";

describe("splitColumns", () => {
  it("regroups rows into columns", () => {
    expect(splitColumns(parsePastedText("1\t10\t100\n2\t20\t200"))).toEqual([
      [1, 2],
      [10, 20],
      [100, 200],
    ]);
  });

  it("keeps a single column", () => {
    expect(splitColumns(parsePastedText("1.23\n1.25"))).toEqual([[1.23, 1.25]]);
  });

  it.each([
    ["a parse error", parsePastedText("1\nabc")],
    ["no rows", parsePastedText("")],
  ])("is empty for %s", (_, parsed) => {
    expect(splitColumns(parsed)).toEqual([]);
  });

  // Pinned: a cell the parser let through but Number cannot read becomes
  // NaN rather than disappearing (the parser rejects these first today).
  it("turns an unreadable cell into NaN", () => {
    expect(splitColumns({ rows: [["1"], ["x"]], columnCount: 1 })).toEqual([
      [1, NaN],
    ]);
  });
});

describe("formatMean", () => {
  it("rounds to the sem's leading digit", () => {
    expect(formatMean(1.2463, 0.025)).toBe("1.25");
  });

  // No sem (a single value) is passed on as -1, which the rounder treats as
  // "no uncertainty": its fixed four decimals.
  it("uses the rounder's default without a sem", () => {
    expect(formatMean(1.23, null)).toBe("1.2300");
  });

  it("uses the same default for a zero sem", () => {
    expect(formatMean(1.23, 0)).toBe("1.2300");
  });

  it.each([NaN, Infinity, -Infinity])("shows %s as '-'", (mean) => {
    expect(formatMean(mean, 0.1)).toBe("-");
  });
});

describe("summarizeColumn", () => {
  it("reports n, mean ± sem and the standard deviation", () => {
    // mean 1.245, stdev 0.05, sem 0.025
    expect(summarizeColumn([1.23, 1.25, 1.19, 1.31])).toEqual({
      n: 4,
      mean: expect.stringMatching(/^1\.2[45] ± 0\.03$/),
      stdev: "0.05",
    });
  });

  it("has no ± and no spread for a single value", () => {
    expect(summarizeColumn([1.23])).toEqual({
      n: 1,
      mean: "1.2300",
      stdev: "-",
    });
  });

  // Identical values: a spread of exactly zero, reported as such, and a
  // mean with nothing to be ± about.
  it("has no ± when every value is the same", () => {
    expect(summarizeColumn([2, 2, 2])).toEqual({
      n: 3,
      mean: "2.0000",
      stdev: "0.0",
    });
  });
});

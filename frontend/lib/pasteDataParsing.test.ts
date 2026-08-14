import { describe, expect, it } from "vitest";
import { buildColumns, parsePastedText } from "./pasteDataParsing";

describe("parsePastedText", () => {
  it("splits tab-delimited rows (spreadsheet paste)", () => {
    const result = parsePastedText("0\t1\n1\t3\n2\t5");
    expect(result.error).toBeUndefined();
    expect(result.columnCount).toBe(2);
    expect(result.rows).toEqual([
      ["0", "1"],
      ["1", "3"],
      ["2", "5"],
    ]);
  });

  it("splits comma-delimited rows (CSV paste)", () => {
    const result = parsePastedText("0,1\n1,3");
    expect(result.error).toBeUndefined();
    expect(result.rows).toEqual([
      ["0", "1"],
      ["1", "3"],
    ]);
  });

  it("falls back to whitespace splitting when no tab or comma is present", () => {
    const result = parsePastedText("0 1\n1   3");
    expect(result.error).toBeUndefined();
    expect(result.rows).toEqual([
      ["0", "1"],
      ["1", "3"],
    ]);
  });

  it("ignores blank lines", () => {
    const result = parsePastedText("0\t1\n\n1\t3\n");
    expect(result.error).toBeUndefined();
    expect(result.rows).toHaveLength(2);
  });

  it("returns empty result for empty input", () => {
    const result = parsePastedText("   \n  ");
    expect(result).toEqual({ rows: [], columnCount: 0 });
  });

  it("errors when row lengths are inconsistent", () => {
    const result = parsePastedText("0\t1\n1\t3\t5");
    expect(result.error).toMatch(/列数が行によって異なります/);
  });

  it("errors when a cell isn't numeric", () => {
    const result = parsePastedText("0\tabc");
    expect(result.error).toMatch(/数値として読めない値があります/);
  });
});

describe("buildColumns", () => {
  it("returns null when parsing failed", () => {
    const parsed = parsePastedText("0\tabc");
    expect(buildColumns(parsed, {}, {})).toBeNull();
  });

  it("returns null when there is no data", () => {
    const parsed = parsePastedText("");
    expect(buildColumns(parsed, {}, {})).toBeNull();
  });

  it("builds x/y columns with no extra columns", () => {
    const parsed = parsePastedText("0\t1\n1\t3\n2\t5");
    expect(buildColumns(parsed, {}, {})).toEqual({
      x: [0, 1, 2],
      y: [1, 3, 5],
    });
  });

  it("maps a role column (e.g. y_error) by name", () => {
    const parsed = parsePastedText("0\t1\t0.1\n1\t3\t0.2");
    const columns = buildColumns(parsed, { 2: "y_error" }, {});
    expect(columns).toEqual({
      x: [0, 1],
      y: [1, 3],
      y_error: [0.1, 0.2],
    });
  });

  it("uses the trimmed custom name for __custom__ role columns", () => {
    const parsed = parsePastedText("0\t1\t9\n1\t3\t8");
    const columns = buildColumns(parsed, { 2: "__custom__" }, { 2: "  weight " });
    expect(columns).toEqual({
      x: [0, 1],
      y: [1, 3],
      weight: [9, 8],
    });
  });

  it("drops a __custom__ column with a blank name", () => {
    const parsed = parsePastedText("0\t1\t9\n1\t3\t8");
    const columns = buildColumns(parsed, { 2: "__custom__" }, { 2: "   " });
    expect(columns).toEqual({ x: [0, 1], y: [1, 3] });
  });

  it("ignores a column left unassigned (defaults to __ignore__)", () => {
    const parsed = parsePastedText("0\t1\t9\n1\t3\t8");
    expect(buildColumns(parsed, {}, {})).toEqual({ x: [0, 1], y: [1, 3] });
  });
});

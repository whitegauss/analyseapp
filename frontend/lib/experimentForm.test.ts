import { describe, expect, it } from "vitest";
import { parseColumnsField } from "./experimentForm";

describe("parseColumnsField", () => {
  it("accepts a well-formed column map", () => {
    const result = parseColumnsField('{"x":[1,2],"y":[3,4]}');
    expect(result).toEqual({ ok: true, columns: { x: [1, 2], y: [3, 4] } });
  });

  it("keeps extra columns alongside x and y", () => {
    const result = parseColumnsField('{"x":[1],"y":[2],"y_error":[0.1]}');
    expect(result.ok && result.columns.y_error).toEqual([0.1]);
  });

  it.each([
    ["the field is absent", null],
    ["the field is empty", ""],
    ["the value is not JSON", "not json"],
    ["the JSON is truncated", '{"x":[1,2]'],
  ])("reports a paste problem when %s", (_label, value) => {
    expect(parseColumnsField(value)).toEqual({
      ok: false,
      error: "データが正しく貼り付けられていません",
    });
  });

  it.each([
    ["x is missing", '{"y":[1,2]}'],
    ["y is missing", '{"x":[1,2]}'],
    ["both are missing", "{}"],
    ["x is null", '{"x":null,"y":[1]}'],
    ["x is 0", '{"x":0,"y":[1]}'],
  ])("reports a missing-column problem when %s", (_label, value) => {
    expect(parseColumnsField(value)).toEqual({
      ok: false,
      error: "x列とy列のデータが必要です",
    });
  });

  // JSON.parse accepts bare scalars, so the shape check meets values that are
  // not objects at all. A number or string reads as "no x column".
  it.each([
    ["a number", "42"],
    ["a string", '"nope"'],
  ])("rejects valid JSON that is %s", (_label, value) => {
    expect(parseColumnsField(value).ok).toBe(false);
  });

  // Current behaviour, pinned rather than endorsed. An empty array is truthy,
  // so a column with no data passes the "x列とy列のデータが必要です" check and
  // an experiment can be created with nothing in it. Fixing this is KAN-61.
  it.each([
    ["x is empty", '{"x":[],"y":[1]}'],
    ["y is empty", '{"x":[1],"y":[]}'],
    ["both are empty", '{"x":[],"y":[]}'],
  ])("today accepts a column map where %s", (_label, value) => {
    expect(parseColumnsField(value).ok).toBe(true);
  });

  // Current behaviour, pinned rather than endorsed. The truthiness check never
  // looks at the shape, so a string or an object passes and ends up inside a
  // Record<string, number[]> — the returned type is a lie. Same root cause as
  // the empty-array case above. Fixing this is KAN-61.
  it.each([
    ["x is a string", '{"x":"invalid","y":[1]}'],
    ["x holds strings instead of numbers", '{"x":["a"],"y":[1]}'],
    ["x is an object", '{"x":{"0":1},"y":[1]}'],
  ])("today accepts a column map where %s", (_label, value) => {
    expect(parseColumnsField(value).ok).toBe(true);
  });

  // Current behaviour, pinned rather than endorsed. try/catch wraps only the
  // JSON.parse call, so reading .x off a parsed null throws a TypeError out of
  // the function instead of returning the friendly error. Fixing this is KAN-61.
  it("today throws instead of reporting an error for the literal null", () => {
    expect(() => parseColumnsField("null")).toThrow(TypeError);
  });
});

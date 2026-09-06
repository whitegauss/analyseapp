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

  // These all used to pass. `!columns.x` only asks whether the value is
  // truthy, and `[]`, `"invalid"` and `{}` all are — so an experiment with no
  // data could be created, and a string could sit inside a
  // Record<string, number[]>, making the return type a lie (KAN-61).
  it.each([
    ["x is empty", '{"x":[],"y":[1]}'],
    ["y is empty", '{"x":[1],"y":[]}'],
    ["both are empty", '{"x":[],"y":[]}'],
    ["x is a string", '{"x":"invalid","y":[1]}'],
    ["x holds strings instead of numbers", '{"x":["a"],"y":[1]}'],
    ["x holds a null", '{"x":[1,null],"y":[1,2]}'],
    ["x is an object", '{"x":{"0":1},"y":[1]}'],
    ["x is a nested array", '{"x":[[1]],"y":[1]}'],
    // JSON has no NaN/Infinity literal, but 1e999 overflows to Infinity.
    ["x overflows to Infinity", '{"x":[1e999],"y":[1]}'],
  ])("rejects a column map where %s", (_label, value) => {
    expect(parseColumnsField(value)).toEqual({
      ok: false,
      error: "x列とy列のデータが必要です",
    });
  });

  // try/catch used to wrap only the JSON.parse call, so reading .x off a
  // parsed null threw a TypeError out of the server action and the user met
  // Next.js's error page instead of the message (KAN-61).
  it.each([
    ["the literal null", "null"],
    ["a number", "42"],
    ["a string", '"nope"'],
    ["a boolean", "true"],
    ["a top-level array", "[1,2]"],
  ])("reports a paste problem for %s rather than throwing", (_label, value) => {
    expect(parseColumnsField(value)).toEqual({
      ok: false,
      error: "データが正しく貼り付けられていません",
    });
  });

  // Extra columns go through the same predicate as x and y. Skipping them
  // would leave the return type lying about exactly the columns nobody
  // thought to check.
  it.each([
    ["y_error is a string", '{"x":[1],"y":[2],"y_error":"0.1"}'],
    ["y_error holds strings", '{"x":[1],"y":[2],"y_error":["0.1"]}'],
    ["y_error is empty", '{"x":[1],"y":[2],"y_error":[]}'],
  ])("rejects an extra column when %s", (_label, value) => {
    const result = parseColumnsField(value);
    expect(result.ok).toBe(false);
    // The message has to name the offending column: with several extra
    // columns, "something is wrong" is not actionable.
    expect(result.ok === false && result.error).toContain("y_error列");
  });

  // The worker's DataSeries.check_equal_length already rejects these, so this
  // is a deliberate duplication: catching it here saves a round trip and
  // keeps a ragged column map out of the database.
  it.each([
    ["y is shorter than x", '{"x":[1,2],"y":[1]}'],
    ["y is longer than x", '{"x":[1],"y":[1,2]}'],
    ["an extra column is ragged", '{"x":[1,2],"y":[1,2],"y_error":[0.1]}'],
  ])("rejects a ragged column map when %s", (_label, value) => {
    expect(parseColumnsField(value)).toEqual({
      ok: false,
      error: "すべての列の行数が揃っている必要があります",
    });
  });

  it("accepts a well-formed map with extra columns of equal length", () => {
    const result = parseColumnsField(
      '{"x":[1,2],"y":[3,4],"y_error":[0.1,0.2]}',
    );
    expect(result).toEqual({
      ok: true,
      columns: { x: [1, 2], y: [3, 4], y_error: [0.1, 0.2] },
    });
  });

  it("keeps a column named __proto__ instead of silently dropping it", () => {
    // JSON.parse gives "__proto__" as a real own property, so it passes the
    // shape check like any other column. Copying it into a normal object
    // then hits Object.prototype's setter rather than defining a property:
    // the column vanishes from the result and the map's prototype is
    // replaced by the array. Object.prototype itself is never touched, so
    // what this guards is the silent data loss.
    const result = parseColumnsField('{"x":[1],"y":[2],"__proto__":[3]}');

    expect(result.ok).toBe(true);
    if (!result.ok) return;
    // Bracket access and Object.keys both read own properties, which is what
    // makes the difference visible -- `.__proto__` on a plain object would
    // follow the prototype instead.
    expect(Object.keys(result.columns).sort()).toEqual(["__proto__", "x", "y"]);
    expect(result.columns["__proto__"]).toEqual([3]);
    // The map has to stay a plain string->number[] map, not become an array.
    expect(Array.isArray(result.columns)).toBe(false);
    // And it must survive the trip to the API, which is a JSON.stringify.
    expect(JSON.parse(JSON.stringify(result.columns))["__proto__"]).toEqual([
      3,
    ]);
  });

  it("rejects a __proto__ column that is not valid data", () => {
    // The same predicate has to apply to it as to every other column.
    expect(parseColumnsField('{"x":[1],"y":[2],"__proto__":"nope"}').ok).toBe(
      false,
    );
  });

  it("accepts negative and fractional values", () => {
    // Nothing here is about the sign or magnitude of the data -- only its
    // shape -- so ordinary physics numbers must still go through.
    const result = parseColumnsField('{"x":[-1.5,0,2e3],"y":[0.001,-4,5]}');
    expect(result.ok).toBe(true);
  });
});

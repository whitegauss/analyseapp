import { describe, expect, it } from "vitest";
import { orderedColumnKeys } from "./columns";

describe("orderedColumnKeys", () => {
  it("puts x and y first, then extras in insertion order", () => {
    expect(orderedColumnKeys({ y_error: [], y: [], x: [], depth: [] })).toEqual(
      ["x", "y", "y_error", "depth"],
    );
  });

  it("keeps x and y in that order even when y was inserted first", () => {
    expect(orderedColumnKeys({ y: [], x: [] })).toEqual(["x", "y"]);
  });

  it("omits x or y when the column is absent", () => {
    expect(orderedColumnKeys({ y: [], y_error: [] })).toEqual(["y", "y_error"]);
    expect(orderedColumnKeys({ x: [] })).toEqual(["x"]);
  });

  it("returns an empty list for an empty map", () => {
    expect(orderedColumnKeys({})).toEqual([]);
  });

  it("keeps a column whose data is empty", () => {
    // Presence is decided by the key, not by whether it holds any values, so a
    // column the user cleared still gets a CSV header.
    expect(orderedColumnKeys({ x: [], y: [] })).toEqual(["x", "y"]);
  });

  it("does not treat a key named like an inherited property as present", () => {
    expect(orderedColumnKeys({ x: [], y: [] })).not.toContain("toString");
  });
});

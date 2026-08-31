import { describe, expect, it } from "vitest";
import { parseCompareIds } from "./compareParams";

describe("parseCompareIds", () => {
  it("splits a comma-separated list", () => {
    expect(parseCompareIds("a,b,c")).toEqual(["a", "b", "c"]);
  });

  it("trims whitespace around each id", () => {
    expect(parseCompareIds(" a , b ")).toEqual(["a", "b"]);
  });

  it("drops empty entries from trailing or doubled commas", () => {
    expect(parseCompareIds("a,,b,")).toEqual(["a", "b"]);
  });

  it.each([
    ["undefined", undefined],
    ["empty", ""],
    ["only separators", ",,,"],
    ["only whitespace", "  ,  "],
  ])("returns an empty list when the param is %s", (_label, value) => {
    expect(parseCompareIds(value)).toEqual([]);
  });

  it("keeps a single id", () => {
    expect(parseCompareIds("only-one")).toEqual(["only-one"]);
  });
});

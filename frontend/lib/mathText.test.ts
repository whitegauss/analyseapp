import { describe, expect, it } from "vitest";
import { parseSegments } from "./mathText";

describe("parseSegments", () => {
  it("returns a single plain segment when there is no math", () => {
    expect(parseSegments("速度")).toEqual([{ math: false, content: "速度" }]);
  });

  it("splits text around a math span", () => {
    expect(parseSegments("速度 $v$ (m/s)")).toEqual([
      { math: false, content: "速度 " },
      { math: true, content: "v" },
      { math: false, content: " (m/s)" },
    ]);
  });

  it("handles a label that is entirely math", () => {
    expect(parseSegments("$v$")).toEqual([{ math: true, content: "v" }]);
  });

  it("handles several math spans", () => {
    expect(parseSegments("$x$ vs $y$")).toEqual([
      { math: true, content: "x" },
      { math: false, content: " vs " },
      { math: true, content: "y" },
    ]);
  });

  it.each([
    ["a bare trailing delimiter", "$v", "$v"],
    ["a price, which must not become math", "cost: $5", "cost: $5"],
  ])("keeps %s as plain text", (_label, input, want) => {
    expect(parseSegments(input)).toEqual([{ math: false, content: want }]);
  });

  it("returns nothing for an empty label", () => {
    expect(parseSegments("")).toEqual([]);
  });

  it("does not treat an empty math span as math", () => {
    // The pattern requires at least one character between the delimiters.
    expect(parseSegments("$$")).toEqual([{ math: false, content: "$$" }]);
  });

  it("preserves LaTeX markup inside the delimiters", () => {
    expect(parseSegments("$\\mathrm{m/s}$")).toEqual([
      { math: true, content: "\\mathrm{m/s}" },
    ]);
  });
});

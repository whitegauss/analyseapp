import { describe, expect, it } from "vitest";
import { parseSegments } from "./AxisLabel";

describe("parseSegments", () => {
  it("returns a single plain-text segment when there is no $...$", () => {
    expect(parseSegments("速度")).toEqual([{ math: false, content: "速度" }]);
  });

  it("extracts a single math segment", () => {
    expect(parseSegments("$v$")).toEqual([{ math: true, content: "v" }]);
  });

  it("splits text-math-text", () => {
    expect(parseSegments("速度 $v$ (m/s)")).toEqual([
      { math: false, content: "速度 " },
      { math: true, content: "v" },
      { math: false, content: " (m/s)" },
    ]);
  });

  it("handles multiple math segments", () => {
    expect(parseSegments("$a$+$b$")).toEqual([
      { math: true, content: "a" },
      { math: false, content: "+" },
      { math: true, content: "b" },
    ]);
  });

  it("treats an unclosed $ as plain text", () => {
    expect(parseSegments("cost: $5")).toEqual([{ math: false, content: "cost: $5" }]);
  });

  it("returns no segments for an empty string", () => {
    expect(parseSegments("")).toEqual([]);
  });
});

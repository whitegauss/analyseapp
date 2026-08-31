import { describe, expect, it } from "vitest";
import { readAxisLabel } from "./experiment";

describe("readAxisLabel", () => {
  it("returns the label when the config holds a string", () => {
    expect(
      readAxisLabel({ x_axis_label: "速度 $v$ (m/s)" }, "x_axis_label"),
    ).toBe("速度 $v$ (m/s)");
  });

  it("returns an empty string when the key is absent", () => {
    expect(readAxisLabel({}, "x_axis_label")).toBe("");
  });

  it("keeps an explicitly empty label as empty", () => {
    expect(readAxisLabel({ x_axis_label: "" }, "x_axis_label")).toBe("");
  });

  it.each([
    ["null", null],
    ["a number", 42],
    ["an object", { text: "v" }],
    ["an array", ["v"]],
    ["undefined", undefined],
  ])("returns an empty string when the value is %s", (_label, value) => {
    expect(readAxisLabel({ x_axis_label: value }, "x_axis_label")).toBe("");
  });

  it("reads each axis independently", () => {
    const config = { x_axis_label: "t (s)", y_axis_label: "v (m/s)" };
    expect(readAxisLabel(config, "x_axis_label")).toBe("t (s)");
    expect(readAxisLabel(config, "y_axis_label")).toBe("v (m/s)");
  });
});

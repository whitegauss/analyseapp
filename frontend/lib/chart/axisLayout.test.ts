import { describe, expect, it } from "vitest";
import { axisLayout } from "./axisLayout";

describe("axisLayout", () => {
  it("leaves the Plotly title empty when a label was supplied", () => {
    // The label is drawn as a separate KaTeX-capable DOM element instead.
    expect(axisLayout("速度 $v$", "X", false).title).toEqual({ text: "" });
  });

  it.each([
    ["absent", ""],
    ["only whitespace", "   "],
  ])("falls back to the default title when the label is %s", (_l, label) => {
    expect(axisLayout(label, "X", false).title).toEqual({ text: "X" });
  });

  it("switches the axis type with the log flag", () => {
    expect(axisLayout("", "X", true).type).toBe("log");
    expect(axisLayout("", "X", false).type).toBe("linear");
  });

  it("adds minor ticks only on a linear axis", () => {
    // A log axis carries explicit tickvals/ticktext; minor ticks there would
    // just be unlabeled clutter.
    expect(axisLayout("", "X", false).minor).toEqual({
      ticks: "inside",
      showgrid: false,
    });
    expect(axisLayout("", "X", true).minor).toBeUndefined();
  });

  it("keeps the frame styling identical on both scales", () => {
    for (const logScale of [true, false]) {
      const a = axisLayout("", "X", logScale);
      expect(a).toMatchObject({
        showgrid: false,
        zeroline: false,
        showline: true,
        mirror: true,
        ticks: "inside",
      });
    }
  });
});

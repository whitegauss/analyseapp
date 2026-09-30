import { describe, expect, it } from "vitest";
import {
  buildComparisonTraces,
  COLORS,
  pickAxisLabel,
  stripMathDelimiters,
  type ComparedExperiment,
} from "./comparison";

function experiment(
  overrides: Partial<ComparedExperiment> = {},
): ComparedExperiment {
  return {
    id: "a",
    title: "落下運動",
    columns: { x: [1, 2, 4], y: [3, 5, 9] },
    xAxisLabel: "",
    yAxisLabel: "",
    regression: null,
    ...overrides,
  };
}

const fit = {
  slope: 2,
  intercept: 1,
  slope_stderr: 0.1,
  intercept_stderr: 0.1,
  r_squared: 0.99,
  weighted: false,
  x_log: false,
  y_log: false,
};

describe("stripMathDelimiters", () => {
  it.each([
    ["$x$", "x"],
    ["速度 $v$ [m/s]", "速度 v [m/s]"],
    ["時間 (s)", "時間 (s)"],
    ["$", ""],
    ["", ""],
  ])("%j -> %j", (label, want) => {
    expect(stripMathDelimiters(label)).toBe(want);
  });
});

describe("pickAxisLabel", () => {
  const labelled = (...labels: string[]) =>
    labels.map((xAxisLabel, i) => experiment({ id: String(i), xAxisLabel }));

  it.each([
    [
      "the first experiment's, when all have one",
      labelled("t (s)", "時間"),
      "t (s)",
    ],
    ["the first non-empty one", labelled("", "時間"), "時間"],
    ["skipping whitespace-only labels", labelled("   ", "時間"), "時間"],
    ["the fallback when none has one", labelled("", " "), "X"],
    ["the fallback with no experiments", [], "X"],
  ])("picks %s", (_, experiments, want) => {
    expect(pickAxisLabel(experiments, "xAxisLabel", "X")).toBe(want);
  });

  it("reads the key it is asked for", () => {
    const exps = [experiment({ xAxisLabel: "x軸", yAxisLabel: "y軸" })];
    expect(pickAxisLabel(exps, "yAxisLabel", "Y")).toBe("y軸");
  });
});

describe("buildComparisonTraces", () => {
  it("draws only the points for an experiment without a fit", () => {
    const traces = buildComparisonTraces([experiment()]);

    expect(traces).toHaveLength(1);
    expect(traces[0]).toMatchObject({
      x: [1, 2, 4],
      y: [3, 5, 9],
      mode: "markers",
      name: "落下運動",
      legendgroup: "a",
    });
    // In the legend: showlegend is left to Plotly's default.
    expect(traces[0]).not.toHaveProperty("showlegend");
  });

  it("adds the fit line across the experiment's own x range", () => {
    const [, line] = buildComparisonTraces([experiment({ regression: fit })]);

    expect(line).toMatchObject({
      x: [1, 4],
      y: [3, 9], // 2x + 1 at x = 1 and x = 4
      mode: "lines",
      name: "落下運動（回帰直線）",
      legendgroup: "a",
      showlegend: false,
    });
  });

  it("draws no line when there are no points to span", () => {
    const traces = buildComparisonTraces([
      experiment({ columns: { x: [], y: [] }, regression: fit }),
    ]);
    expect(traces).toHaveLength(1);
  });

  it("pairs each experiment's points and line by color and legend group", () => {
    const traces = buildComparisonTraces([
      experiment({ id: "a", regression: fit }),
      experiment({ id: "b" }),
      experiment({ id: "c", regression: fit }),
    ]);

    expect(traces.map((t) => [t.legendgroup, t.mode])).toEqual([
      ["a", "markers"],
      ["a", "lines"],
      ["b", "markers"],
      ["c", "markers"],
      ["c", "lines"],
    ]);
    expect(
      traces.map((t) => (t.mode === "lines" ? t.line?.color : t.marker?.color)),
    ).toEqual([COLORS[0], COLORS[0], COLORS[1], COLORS[2], COLORS[2]]);
  });

  it("cycles the palette once there are more experiments than colors", () => {
    const many = Array.from({ length: COLORS.length + 2 }, (_, i) =>
      experiment({ id: String(i) }),
    );
    const colors = buildComparisonTraces(many).map((t) => t.marker?.color);

    expect(colors[COLORS.length]).toBe(COLORS[0]);
    expect(colors[COLORS.length + 1]).toBe(COLORS[1]);
  });
});

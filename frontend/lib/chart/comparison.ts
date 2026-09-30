import type { PlotData } from "plotly.js";
import type { LinearRegressionResult } from "@/lib/experiment";

// What the comparison chart draws, built without Plotly or React so the
// rules -- which color, which traces, which axis title -- are testable on
// their own (KAN-49). components/ComparisonChart.tsx only hands the result
// to <Plot>.

export type ComparedExperiment = {
  id: string;
  title: string;
  columns: Record<string, number[]>;
  xAxisLabel: string;
  yAxisLabel: string;
  // Always the linear-scale (x_log=false, y_log=false) fit -- the compare
  // view doesn't offer a log-axis toggle, so a plain y = slope*x + intercept
  // line is exact here.
  regression: LinearRegressionResult | null;
};

// A fixed, high-contrast palette (Plotly's default qualitative sequence)
// cycled by index -- distinguishing an unbounded number of experiments by
// color alone stops being reliable well before this list runs out, but
// that's an acceptable limit for an overlay comparison chart.
export const COLORS = [
  "#1f77b4",
  "#ff7f0e",
  "#2ca02c",
  "#d62728",
  "#9467bd",
  "#8c564b",
  "#e377c2",
  "#7f7f7f",
  "#bcbd22",
  "#17becf",
];

/**
 * Plotly axis titles don't render KaTeX (unlike the single-experiment
 * ExperimentChart, which overlays a separate KaTeX-rendered label instead of
 * using Plotly's own title). Stripping the $...$ delimiters keeps the text
 * readable as plain text rather than showing literal dollar signs.
 */
export function stripMathDelimiters(label: string): string {
  return label.replace(/\$/g, "");
}

/**
 * The axis title for the whole chart: the first experiment's label that is
 * not blank, else `fallback`. Experiments labelled differently are not
 * reconciled -- the first one wins.
 */
export function pickAxisLabel(
  experiments: ComparedExperiment[],
  key: "xAxisLabel" | "yAxisLabel",
  fallback: string,
): string {
  return experiments.find((e) => e[key].trim())?.[key] ?? fallback;
}

/**
 * One scatter trace per experiment, followed by its fit line when it has a
 * fit and any points. The two share a color and a `legendgroup`, so the
 * legend shows one entry per experiment and toggling it hides both; the
 * line itself stays out of the legend. The line spans the experiment's own
 * x range, not the plot's -- other experiments' ranges say nothing about
 * where this fit holds.
 */
export function buildComparisonTraces(
  experiments: ComparedExperiment[],
): Partial<PlotData>[] {
  const data: Partial<PlotData>[] = [];

  experiments.forEach((exp, i) => {
    const color = COLORS[i % COLORS.length];
    const { x, y } = exp.columns;

    data.push({
      x,
      y,
      type: "scatter",
      mode: "markers",
      marker: { size: 7, color },
      name: exp.title,
      legendgroup: exp.id,
    });

    if (exp.regression && x.length > 0) {
      const xMin = Math.min(...x);
      const xMax = Math.max(...x);
      const { slope, intercept } = exp.regression;
      data.push({
        x: [xMin, xMax],
        y: [slope * xMin + intercept, slope * xMax + intercept],
        type: "scatter",
        mode: "lines",
        line: { color, dash: "dash", width: 1.5 },
        name: `${exp.title}（回帰直線）`,
        legendgroup: exp.id,
        showlegend: false,
      });
    }
  });

  return data;
}

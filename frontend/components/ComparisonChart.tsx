"use client";

import dynamic from "next/dynamic";
import type { Data } from "plotly.js";
import ChartSkeleton from "./ChartSkeleton";
import type { LinearRegressionResult } from "./ExperimentChart";

const Plot = dynamic(() => import("react-plotly.js"), {
  ssr: false,
  loading: () => <ChartSkeleton />,
});

// A fixed, high-contrast palette (Plotly's default qualitative sequence)
// cycled by index -- distinguishing an unbounded number of experiments by
// color alone stops being reliable well before this list runs out, but
// that's an acceptable limit for an overlay comparison chart.
const COLORS = [
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

// Plotly axis titles don't render KaTeX (unlike the single-experiment
// ExperimentChart, which overlays a separate KaTeX-rendered label instead of
// using Plotly's own title). Stripping the $...$ delimiters keeps the text
// readable as plain text rather than showing literal dollar signs.
function stripMathDelimiters(label: string): string {
  return label.replace(/\$/g, "");
}

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

type Props = {
  experiments: ComparedExperiment[];
};

export default function ComparisonChart({ experiments }: Props) {
  const data: Partial<Data>[] = [];

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

  const xAxisLabel =
    experiments.find((e) => e.xAxisLabel.trim())?.xAxisLabel ?? "X";
  const yAxisLabel =
    experiments.find((e) => e.yAxisLabel.trim())?.yAxisLabel ?? "Y";

  return (
    <Plot
      data={data}
      layout={{
        autosize: true,
        margin: { t: 20, r: 20, b: 50, l: 60 },
        xaxis: {
          title: { text: stripMathDelimiters(xAxisLabel) },
          showgrid: false,
          zeroline: false,
          showline: true,
          mirror: true,
          ticks: "inside",
        },
        yaxis: {
          title: { text: stripMathDelimiters(yAxisLabel) },
          showgrid: false,
          zeroline: false,
          showline: true,
          mirror: true,
          ticks: "inside",
        },
        legend: { orientation: "h", y: -0.2 },
      }}
      style={{ width: "100%", height: "480px" }}
      useResizeHandler
      config={{ displaylogo: false }}
    />
  );
}

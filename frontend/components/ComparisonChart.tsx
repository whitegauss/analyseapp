"use client";

import dynamic from "next/dynamic";
import ChartSkeleton from "./ChartSkeleton";
import {
  buildComparisonTraces,
  pickAxisLabel,
  stripMathDelimiters,
  type ComparedExperiment,
} from "@/lib/chart/comparison";

const Plot = dynamic(() => import("react-plotly.js"), {
  ssr: false,
  loading: () => <ChartSkeleton />,
});

type Props = {
  experiments: ComparedExperiment[];
};

export default function ComparisonChart({ experiments }: Props) {
  const data = buildComparisonTraces(experiments);
  const xAxisLabel = pickAxisLabel(experiments, "xAxisLabel", "X");
  const yAxisLabel = pickAxisLabel(experiments, "yAxisLabel", "Y");

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

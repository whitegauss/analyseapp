"use client";

import dynamic from "next/dynamic";
import type { Data } from "plotly.js";
import ChartSkeleton from "./ChartSkeleton";
import AxisLabelRuns, { type AxisLabelRun } from "./AxisLabelRuns";

// plotly.js touches `window`, so it can only load in the browser. While the
// library itself is being fetched, show the same skeleton used for "no data
// yet" so there's no visible jump.
const Plot = dynamic(() => import("react-plotly.js"), {
  ssr: false,
  loading: () => <ChartSkeleton />,
});

type Props = {
  columns: Record<string, number[]>;
  title: string;
  xAxisLabelRuns?: AxisLabelRun[];
  yAxisLabelRuns?: AxisLabelRun[];
};

export default function ExperimentChart({
  columns,
  title,
  xAxisLabelRuns = [],
  yAxisLabelRuns = [],
}: Props) {
  const { x, y, ...rest } = columns;

  const trace: Partial<Data> = {
    x,
    y,
    type: "scatter",
    mode: "markers",
    marker: { size: 8 },
  };

  if (rest.y_error) {
    trace.error_y = { type: "data", array: rest.y_error, visible: true };
  }
  if (rest.x_error) {
    trace.error_x = { type: "data", array: rest.x_error, visible: true };
  }

  return (
    <div className="flex flex-col items-center gap-1">
      <div className="flex w-full items-center gap-2">
        {yAxisLabelRuns.length > 0 && (
          <AxisLabelRuns runs={yAxisLabelRuns} vertical />
        )}
        <div className="min-w-0 flex-1">
          <Plot
            data={[trace]}
            layout={{
              title: { text: title },
              xaxis: {
                title: { text: xAxisLabelRuns.length > 0 ? "" : "x" },
                showgrid: false,
                showline: true,
                mirror: true,
                ticks: "inside",
                minor: { ticks: "inside", showgrid: false },
              },
              yaxis: {
                title: { text: yAxisLabelRuns.length > 0 ? "" : "y" },
                showgrid: false,
                showline: true,
                mirror: true,
                ticks: "inside",
                minor: { ticks: "inside", showgrid: false },
              },
              autosize: true,
              margin: { t: 48, r: 24, b: 48, l: 56 },
            }}
            style={{ width: "100%", height: "480px" }}
            useResizeHandler
            config={{ responsive: true }}
          />
        </div>
      </div>
      {xAxisLabelRuns.length > 0 && <AxisLabelRuns runs={xAxisLabelRuns} />}
    </div>
  );
}

"use client";

import { useMemo, useState } from "react";
import dynamic from "next/dynamic";
import type { Data } from "plotly.js";
import ChartSkeleton from "./ChartSkeleton";
import AxisLabel from "./AxisLabel";

// plotly.js touches `window`, so it can only load in the browser. While the
// library itself is being fetched, show the same skeleton used for "no data
// yet" so there's no visible jump.
const Plot = dynamic(() => import("react-plotly.js"), {
  ssr: false,
  loading: () => <ChartSkeleton />,
});

export type LinearRegressionResult = {
  slope: number;
  intercept: number;
  slope_stderr: number;
  intercept_stderr: number;
  r_squared: number;
  weighted: boolean;
};

type Props = {
  columns: Record<string, number[]>;
  title: string;
  xAxisLabel?: string;
  yAxisLabel?: string;
  regression?: LinearRegressionResult | null;
};

function axisLayout(label: string, fallbackTitle: string) {
  return {
    title: { text: label.trim() ? "" : fallbackTitle },
    showgrid: false,
    zeroline: false,
    showline: true,
    mirror: true,
    ticks: "inside" as const,
    minor: { ticks: "inside" as const, showgrid: false },
  };
}

export default function ExperimentChart({
  columns,
  title,
  xAxisLabel = "",
  yAxisLabel = "",
  regression = null,
}: Props) {
  const [showRegression, setShowRegression] = useState(true);
  const [legendFontSize, setLegendFontSize] = useState(12);
  const { x, y, ...rest } = columns;

  const trace: Partial<Data> = {
    x,
    y,
    type: "scatter",
    mode: "markers",
    marker: { size: 8 },
    name: "データ",
  };

  if (rest.y_error) {
    trace.error_y = { type: "data", array: rest.y_error, visible: true };
  }
  if (rest.x_error) {
    trace.error_x = { type: "data", array: rest.x_error, visible: true };
  }

  // A straight line only needs its two endpoints -- computing y at every x
  // (rather than reusing the worker's per-point predicted_y) also avoids a
  // zig-zag line if x isn't sorted in the pasted data.
  const regressionTrace: Partial<Data> | null = useMemo(() => {
    if (!regression || !showRegression || x.length === 0) return null;
    const xMin = Math.min(...x);
    const xMax = Math.max(...x);
    return {
      x: [xMin, xMax],
      y: [
        regression.slope * xMin + regression.intercept,
        regression.slope * xMax + regression.intercept,
      ],
      type: "scatter",
      mode: "lines",
      name: "回帰直線",
      line: { color: "#ef4444" },
    };
  }, [regression, showRegression, x]);

  return (
    <div className="flex flex-col items-center gap-1">
      <div className="flex w-full items-center gap-2">
        {yAxisLabel.trim() && <AxisLabel label={yAxisLabel} vertical />}
        <div className="min-w-0 flex-1">
          <Plot
            data={regressionTrace ? [trace, regressionTrace] : [trace]}
            layout={{
              title: { text: title },
              xaxis: axisLayout(xAxisLabel, "x"),
              yaxis: axisLayout(yAxisLabel, "y"),
              showlegend: true,
              legend: { font: { size: legendFontSize } },
              autosize: true,
              margin: { t: 48, r: 24, b: 48, l: 56 },
            }}
            style={{ width: "100%", height: "480px" }}
            useResizeHandler
            config={{ responsive: true, edits: { legendPosition: true } }}
          />
        </div>
      </div>
      {xAxisLabel.trim() && <AxisLabel label={xAxisLabel} />}

      <label className="flex items-center gap-2 text-xs text-zinc-600 dark:text-zinc-400">
        凡例の文字サイズ
        <input
          type="range"
          min={8}
          max={24}
          value={legendFontSize}
          onChange={(e) => setLegendFontSize(Number(e.target.value))}
        />
        <span className="w-8 text-right">{legendFontSize}px</span>
      </label>

      {regression && (
        <div className="flex flex-col items-center gap-1 text-sm text-zinc-600 dark:text-zinc-400">
          <label className="flex items-center gap-1.5">
            <input
              type="checkbox"
              checked={showRegression}
              onChange={(e) => setShowRegression(e.target.checked)}
            />
            回帰直線を表示
          </label>
          {showRegression && (
            <p>
              y = {regression.slope.toFixed(4)}x +{" "}
              {regression.intercept.toFixed(4)}
              {"　"}(slope誤差 ±{regression.slope_stderr.toFixed(4)}, R² ={" "}
              {regression.r_squared.toFixed(4)}
              {regression.weighted ? "、誤差重み付き" : ""})
            </p>
          )}
        </div>
      )}
    </div>
  );
}

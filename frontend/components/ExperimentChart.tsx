"use client";

import { useMemo, useState } from "react";
import dynamic from "next/dynamic";
import type { Data } from "plotly.js";
import ChartSkeleton from "./ChartSkeleton";
import AxisLabel from "./AxisLabel";
import { linearRange, logRange, ticksForLogRange } from "@/lib/chart/logScale";
import { axisLayout } from "@/lib/chart/axisLayout";
import {
  evaluateModel,
  formatModelEquation,
  formatSigned,
  regressionCacheKey,
  uncertaintyBoundsAt,
  UNKNOWN_UNCERTAINTY_TEXT,
} from "@/lib/chart/regression";
import { fetchRegression } from "@/app/experiments/actions";
import {
  formatUncertainty,
  roundToUncertainty,
} from "@/lib/significantFigures";
import type { LinearRegressionResult } from "@/lib/experiment";

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
  xAxisLabel?: string;
  yAxisLabel?: string;
  // Present only for a saved experiment (not the unsaved live preview),
  // where switching to a log axis can re-run the analysis via fetchRegression.
  experimentId?: string;
  // The linear-scale (x_log=false, y_log=false) fit, computed once up front
  // by the page. Log-scale variants are fetched lazily, only if the user
  // actually switches a checkbox on.
  initialRegression?: LinearRegressionResult | null;
};

export default function ExperimentChart({
  columns,
  title,
  xAxisLabel = "",
  yAxisLabel = "",
  experimentId,
  initialRegression = null,
}: Props) {
  const [showRegression, setShowRegression] = useState(true);
  const [legendFontSize, setLegendFontSize] = useState(12);
  const [xLogScale, setXLogScale] = useState(false);
  const [yLogScale, setYLogScale] = useState(false);
  const [logRegressionCache, setLogRegressionCache] = useState<
    Record<string, LinearRegressionResult | null>
  >({});
  const [pendingLogFetch, setPendingLogFetch] = useState(false);
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

  // Fixed explicitly (rather than left to Plotly's autorange) so the
  // regression line below can be drawn all the way to the plot's edges --
  // autorange would otherwise keep expanding to fit the line's own
  // endpoints, a circular chase that never settles at the frame boundary.
  const xRange: [number, number] | undefined = useMemo(() => {
    return linearRange(x);
  }, [x]);

  // Same idea, but for a log-scaled x/y axis: Plotly interprets a log
  // axis's `range` as log10 of the displayed bounds (range [0, 2] means 1
  // to 100), and only positive values can appear on it at all.
  const xRangeLog = useMemo(() => logRange(x), [x]);
  const yRangeLog = useMemo(() => logRange(y), [y]);

  // logTicks needs real (non-log10) bounds; xRangeLog/yRangeLog are in
  // log10 units (see above), so convert back with 10^x.
  const xLogTicks = useMemo(() => ticksForLogRange(xRangeLog), [xRangeLog]);
  const yLogTicks = useMemo(() => ticksForLogRange(yRangeLog), [yRangeLog]);

  // The regression line's endpoints, as real (non-log) x values -- trace
  // data is always real values even on a log axis, only the `range` layout
  // field above uses log10 units.
  const lineXBounds: [number, number] | undefined = useMemo(() => {
    if (xLogScale) {
      return xRangeLog
        ? [Math.pow(10, xRangeLog[0]), Math.pow(10, xRangeLog[1])]
        : undefined;
    }
    return xRange;
  }, [xLogScale, xRangeLog, xRange]);

  const activeRegression: LinearRegressionResult | null = useMemo(() => {
    if (!xLogScale && !yLogScale) return initialRegression;
    return logRegressionCache[regressionCacheKey(xLogScale, yLogScale)] ?? null;
  }, [xLogScale, yLogScale, initialRegression, logRegressionCache]);

  // Fetches (once, then cached in state) the regression for a given
  // x_log/y_log combination. A no-op in the unsaved-preview case
  // (experimentId unset) or for the linear/linear default, which is already
  // available as initialRegression.
  async function ensureRegressionFor(xLog: boolean, yLog: boolean) {
    if (!xLog && !yLog) return;
    if (!experimentId) return;
    const key = regressionCacheKey(xLog, yLog);
    if (key in logRegressionCache) return;
    setPendingLogFetch(true);
    const result = await fetchRegression(experimentId, {
      x_log: xLog,
      y_log: yLog,
    });
    setLogRegressionCache((prev) => ({ ...prev, [key]: result }));
    setPendingLogFetch(false);
  }

  function handleXLogChange(checked: boolean) {
    setXLogScale(checked);
    void ensureRegressionFor(checked, yLogScale);
  }

  function handleYLogChange(checked: boolean) {
    setYLogScale(checked);
    void ensureRegressionFor(xLogScale, checked);
  }

  // A straight line only needs its two endpoints. Using the axis range's
  // edges (rather than the data's own min/max) makes the line span the full
  // width of the plot frame instead of stopping at the outermost points.
  const regressionTrace: Partial<Data> | null = useMemo(() => {
    if (!activeRegression || !showRegression || !lineXBounds) return null;
    const [lineXMin, lineXMax] = lineXBounds;
    return {
      x: [lineXMin, lineXMax],
      y: [
        evaluateModel(activeRegression, lineXMin, xLogScale, yLogScale),
        evaluateModel(activeRegression, lineXMax, xLogScale, yLogScale),
      ],
      type: "scatter",
      mode: "lines",
      name: formatModelEquation(activeRegression),
      line: { color: "#ef4444" },
    };
  }, [activeRegression, showRegression, lineXBounds, xLogScale, yLogScale]);

  // A ±1σ uncertainty band around the fit line. The worker doesn't expose
  // the slope/intercept covariance, so this isn't a statistically rigorous
  // confidence band -- at each x it just takes the widest spread across all
  // four ±slope_stderr/±intercept_stderr combinations (evaluated in
  // whichever space the fit was computed in, then converted back to real y
  // values if y was log-transformed -- 10^t is monotonic, so taking the
  // min/max before converting is equivalent to after), which is the usual
  // "steepest vs. shallowest plausible line" envelope drawn by hand in an
  // intro physics lab. A zero (or otherwise unusable) stderr collapses the
  // band onto the line itself rather than being treated as an error.
  const regressionBand: Partial<Data>[] | null = useMemo(() => {
    if (!activeRegression || !showRegression || !lineXBounds) return null;
    const [lineXMin, lineXMax] = lineXBounds;
    const boundsAt = (xv: number) =>
      uncertaintyBoundsAt(activeRegression, xv, xLogScale, yLogScale);
    const atMin = boundsAt(lineXMin);
    const atMax = boundsAt(lineXMax);

    const lower: Partial<Data> = {
      x: [lineXMin, lineXMax],
      y: [atMin.lower, atMax.lower],
      type: "scatter",
      mode: "lines",
      line: { width: 0 },
      hoverinfo: "skip",
      showlegend: false,
    };
    const upper: Partial<Data> = {
      x: [lineXMin, lineXMax],
      y: [atMin.upper, atMax.upper],
      type: "scatter",
      mode: "lines",
      line: { width: 0 },
      fill: "tonexty",
      fillcolor: "rgba(239, 68, 68, 0.15)",
      hoverinfo: "skip",
      showlegend: false,
    };
    return [lower, upper];
  }, [activeRegression, showRegression, lineXBounds, xLogScale, yLogScale]);

  return (
    <div className="flex flex-col items-center gap-1">
      <div className="flex w-full items-center gap-2">
        {yAxisLabel.trim() && <AxisLabel label={yAxisLabel} vertical />}
        <div className="min-w-0 flex-1">
          <Plot
            data={[
              ...(regressionBand ?? []),
              trace,
              ...(regressionTrace ? [regressionTrace] : []),
            ]}
            layout={{
              title: { text: title },
              xaxis: {
                ...axisLayout(xAxisLabel, "x", xLogScale),
                range: xLogScale ? xRangeLog : xRange,
                ...(xLogScale && xLogTicks
                  ? {
                      tickmode: "array" as const,
                      tickvals: xLogTicks.tickvals,
                      ticktext: xLogTicks.ticktext,
                    }
                  : {}),
              },
              yaxis: {
                ...axisLayout(yAxisLabel, "y", yLogScale),
                range: yLogScale ? yRangeLog : undefined,
                ...(yLogScale && yLogTicks
                  ? {
                      tickmode: "array" as const,
                      tickvals: yLogTicks.tickvals,
                      ticktext: yLogTicks.ticktext,
                    }
                  : {}),
              },
              // Keeps this stable across re-renders that don't change the
              // underlying data/axis mode (e.g. the legend font-size
              // slider, toggling the regression line) so Plotly doesn't
              // discard user-driven view state -- zoom/pan range, a
              // dragged legend position -- every time layout is recreated.
              // Switching linear<->log axis scale intentionally changes
              // this, since any prior zoom/pan range is meaningless once
              // the axis type itself changes.
              uirevision: JSON.stringify({ columns, xLogScale, yLogScale }),
              showlegend: true,
              legend: {
                font: { size: legendFontSize },
                bordercolor: "#a1a1aa",
                borderwidth: 1,
                bgcolor: "rgba(255,255,255,0.85)",
                x: 0.02,
                y: 0.98,
                xanchor: "left",
                yanchor: "top",
              },
              autosize: true,
              margin: { t: 48, r: 24, b: 48, l: 56 },
            }}
            style={{ width: "100%", height: "480px" }}
            useResizeHandler
            config={{
              responsive: true,
              edits: { legendPosition: true, legendText: true },
            }}
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

      <div className="flex items-center gap-4 text-xs text-zinc-600 dark:text-zinc-400">
        <label className="flex items-center gap-1.5">
          <input
            type="checkbox"
            checked={xLogScale}
            onChange={(e) => handleXLogChange(e.target.checked)}
          />
          X軸を対数表示
        </label>
        <label className="flex items-center gap-1.5">
          <input
            type="checkbox"
            checked={yLogScale}
            onChange={(e) => handleYLogChange(e.target.checked)}
          />
          Y軸を対数表示
        </label>
      </div>

      {initialRegression && (
        <div className="flex flex-col items-center gap-1 text-sm text-zinc-600 dark:text-zinc-400">
          <label className="flex items-center gap-1.5">
            <input
              type="checkbox"
              checked={showRegression}
              onChange={(e) => setShowRegression(e.target.checked)}
            />
            回帰直線を表示
          </label>
          {showRegression &&
            (activeRegression ? (
              (() => {
                const slope = roundToUncertainty(
                  activeRegression.slope,
                  activeRegression.slope_stderr,
                );
                const intercept = roundToUncertainty(
                  activeRegression.intercept,
                  activeRegression.intercept_stderr,
                );
                const lhsText = yLogScale ? "log₁₀(y)" : "y";
                const xText = xLogScale ? "log₁₀(x)" : "x";
                return (
                  <p>
                    {lhsText} = {slope.rounded.toFixed(slope.decimals)}
                    {xText}{" "}
                    {formatSigned(intercept.rounded, intercept.decimals)}
                    {"　"}(
                    {activeRegression.slope_stderr === null ||
                    activeRegression.intercept_stderr === null ? (
                      // A two-point fit pins the line exactly but leaves no
                      // residual to estimate a spread from. "± 0.0" would read
                      // as a perfect measurement, so say what is actually
                      // true instead.
                      <>誤差{UNKNOWN_UNCERTAINTY_TEXT}（2点フィット）, </>
                    ) : (
                      <>
                        slope誤差 ±
                        {formatUncertainty(activeRegression.slope_stderr)}
                        （1σ）, intercept誤差 ±
                        {formatUncertainty(activeRegression.intercept_stderr)}
                        （1σ）,{" "}
                      </>
                    )}
                    R² = {activeRegression.r_squared.toFixed(4)}
                    {(xLogScale || yLogScale) && "（対数変換後）"}
                    {activeRegression.weighted ? "、誤差重み付き" : ""})
                  </p>
                );
              })()
            ) : pendingLogFetch ? (
              <p>解析中...</p>
            ) : (
              <p>
                この軸設定では回帰直線を計算できませんでした（有効な正の値を持つデータ点が不足している可能性があります）
              </p>
            ))}
        </div>
      )}
    </div>
  );
}

"use client";

import { useMemo, useState } from "react";
import dynamic from "next/dynamic";
import type { Data } from "plotly.js";
import ChartSkeleton from "./ChartSkeleton";
import AxisLabel, { parseSegments } from "./AxisLabel";

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

// Plotly legend/title text supports a small subset of HTML (<i>, <b>, etc.)
// but not KaTeX. This reuses AxisLabel's exact $...$ parsing so authoring
// stays consistent with the axis labels, and approximates the math styling
// with Plotly's native italics instead of true KaTeX typesetting.
function mathItalicHtml(text: string): string {
  return parseSegments(text)
    .map((seg) => (seg.math ? `<i>${seg.content}</i>` : seg.content))
    .join("");
}

// Rounds value to the same place as uncertainty's leading (1-significant-
// figure) digit -- standard convention for reporting a measurement
// alongside its 1σ uncertainty. Works symmetrically in both directions:
// a small uncertainty (e.g. 0.05) rounds the value to that decimal place
// (2.03847 -> 2.04), and a large uncertainty (e.g. 52.3) rounds the value
// to that same tens/hundreds/... place instead (517 -> 520). Displayed
// decimal places are additionally capped at 4 (the value is still rounded
// to the true uncertainty digit even when that digit is finer than 4
// decimals) so a tiny uncertainty doesn't blow up the displayed number
// into a long string of digits.
const MAX_DISPLAY_DECIMALS = 4;

export function roundToUncertainty(
  value: number,
  uncertainty: number,
): { rounded: number; decimals: number } {
  if (!Number.isFinite(uncertainty) || uncertainty <= 0) {
    return { rounded: value, decimals: MAX_DISPLAY_DECIMALS };
  }
  const exponent = Math.floor(Math.log10(uncertainty));
  const scale = Math.pow(10, exponent);
  const decimals = Math.min(MAX_DISPLAY_DECIMALS, Math.max(0, -exponent));
  return { rounded: Math.round(value / scale) * scale, decimals };
}

function formatSigned(value: number, decimals: number): string {
  const sign = value < 0 ? "-" : "+";
  return `${sign} ${Math.abs(value).toFixed(decimals)}`;
}

// Formats an uncertainty (stderr) value on its own -- as opposed to
// roundToUncertainty, which rounds some other value *to* an uncertainty.
// A non-positive or non-finite uncertainty (e.g. from floating-point
// cancellation on a near-perfect fit) just displays as "0.0" rather than
// propagating NaN/Infinity or a nonsensical value.
export function formatUncertainty(uncertainty: number): string {
  if (!Number.isFinite(uncertainty) || uncertainty <= 0) return "0.0";
  const { rounded, decimals } = roundToUncertainty(uncertainty, uncertainty);
  return rounded.toFixed(decimals);
}

// A non-positive/non-finite stderr is treated as exactly zero uncertainty
// (see formatUncertainty above) rather than skipping the band entirely.
function safeStderr(stderr: number): number {
  return Number.isFinite(stderr) && stderr > 0 ? stderr : 0;
}

function formatSignedWithUncertainty(
  value: number,
  decimals: number,
  uncertainty: number,
): string {
  const sign = value < 0 ? "-" : "+";
  return `${sign} (${Math.abs(value).toFixed(decimals)} ± ${formatUncertainty(uncertainty)})`;
}

function formatLineEquation(regression: LinearRegressionResult): string {
  const slope = roundToUncertainty(regression.slope, regression.slope_stderr);
  const intercept = roundToUncertainty(
    regression.intercept,
    regression.intercept_stderr,
  );
  const slopeTerm = `(${slope.rounded.toFixed(slope.decimals)} ± ${formatUncertainty(regression.slope_stderr)})`;
  const interceptTerm = formatSignedWithUncertainty(
    intercept.rounded,
    intercept.decimals,
    regression.intercept_stderr,
  );
  const equation = `$y$ = ${slopeTerm}$x$ ${interceptTerm}`;
  return mathItalicHtml(equation);
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

  // Fixed explicitly (rather than left to Plotly's autorange) so the
  // regression line below can be drawn all the way to the plot's edges --
  // autorange would otherwise keep expanding to fit the line's own
  // endpoints, a circular chase that never settles at the frame boundary.
  const xRange: [number, number] | undefined = useMemo(() => {
    if (x.length === 0) return undefined;
    const min = Math.min(...x);
    const max = Math.max(...x);
    const pad = (max - min) * 0.05 || 1;
    return [min - pad, max + pad];
  }, [x]);

  // A straight line only needs its two endpoints. Using the axis range's
  // edges (rather than the data's own min/max) makes the line span the full
  // width of the plot frame instead of stopping at the outermost points.
  const regressionTrace: Partial<Data> | null = useMemo(() => {
    if (!regression || !showRegression || !xRange) return null;
    const [lineXMin, lineXMax] = xRange;
    return {
      x: [lineXMin, lineXMax],
      y: [
        regression.slope * lineXMin + regression.intercept,
        regression.slope * lineXMax + regression.intercept,
      ],
      type: "scatter",
      mode: "lines",
      name: formatLineEquation(regression),
      line: { color: "#ef4444" },
    };
  }, [regression, showRegression, xRange]);

  // A ±1σ uncertainty band around the fit line. The worker doesn't expose
  // the slope/intercept covariance, so this isn't a statistically rigorous
  // confidence band -- at each x it just takes the widest spread across all
  // four ±slope_stderr/±intercept_stderr combinations, which is the usual
  // "steepest vs. shallowest plausible line" envelope drawn by hand in an
  // intro physics lab. A zero (or otherwise unusable) stderr collapses the
  // band onto the line itself rather than being treated as an error.
  const regressionBand: Partial<Data>[] | null = useMemo(() => {
    if (!regression || !showRegression || !xRange) return null;
    const [lineXMin, lineXMax] = xRange;
    const slopeErr = safeStderr(regression.slope_stderr);
    const interceptErr = safeStderr(regression.intercept_stderr);

    const boundsAt = (xv: number) => {
      const candidates = [
        (regression.slope + slopeErr) * xv +
          (regression.intercept + interceptErr),
        (regression.slope + slopeErr) * xv +
          (regression.intercept - interceptErr),
        (regression.slope - slopeErr) * xv +
          (regression.intercept + interceptErr),
        (regression.slope - slopeErr) * xv +
          (regression.intercept - interceptErr),
      ];
      return { upper: Math.max(...candidates), lower: Math.min(...candidates) };
    };
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
  }, [regression, showRegression, xRange]);

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
              xaxis: { ...axisLayout(xAxisLabel, "x"), range: xRange },
              yaxis: axisLayout(yAxisLabel, "y"),
              // Keeps this stable across re-renders that don't change the
              // underlying data (e.g. the legend font-size slider, toggling
              // the regression line) so Plotly doesn't discard user-driven
              // view state -- zoom/pan range, a dragged legend position --
              // every time layout is recreated. It only changes when the
              // actual plotted data does, which is when a reset is wanted.
              uirevision: JSON.stringify(columns),
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
          {showRegression &&
            (() => {
              const slope = roundToUncertainty(
                regression.slope,
                regression.slope_stderr,
              );
              const intercept = roundToUncertainty(
                regression.intercept,
                regression.intercept_stderr,
              );
              return (
                <p>
                  y = {slope.rounded.toFixed(slope.decimals)}x{" "}
                  {formatSigned(intercept.rounded, intercept.decimals)}
                  {"　"}(slope誤差 ±{formatUncertainty(regression.slope_stderr)}
                  （1σ）, intercept誤差 ±
                  {formatUncertainty(regression.intercept_stderr)}（1σ）, R² ={" "}
                  {regression.r_squared.toFixed(4)}
                  {regression.weighted ? "、誤差重み付き" : ""})
                </p>
              );
            })()}
        </div>
      )}
    </div>
  );
}

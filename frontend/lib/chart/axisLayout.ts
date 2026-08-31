/**
 * Plotly axis layout for one axis.
 *
 * The title is left empty when the user supplied a label: Plotly axis titles
 * cannot render KaTeX, so the label is drawn as a separate DOM element
 * (AxisLabel) positioned alongside the plot instead. The fallback title is
 * only used when there is no label to draw.
 */
export function axisLayout(
  label: string,
  fallbackTitle: string,
  logScale: boolean,
) {
  return {
    title: { text: label.trim() ? "" : fallbackTitle },
    type: logScale ? ("log" as const) : ("linear" as const),
    showgrid: false,
    zeroline: false,
    showline: true,
    mirror: true,
    ticks: "inside" as const,
    // A log axis gets explicit tickvals/ticktext (see logTicks) instead; an
    // extra minor-tick layer would only add unlabeled clutter. Minor ticks are
    // a linear-axis-only touch.
    minor: logScale ? undefined : { ticks: "inside" as const, showgrid: false },
  };
}

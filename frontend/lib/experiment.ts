// Domain types and config accessors shared by the experiment pages, the server
// actions, and the chart components.
//
// These live in lib/ rather than alongside a component because server-side
// code (route handlers, server actions, server components) needs them too, and
// importing from a "use client" module just to name a type points the
// dependency the wrong way.

export type LinearRegressionResult = {
  slope: number;
  intercept: number;
  /** null when the fit could not estimate an uncertainty: a two-point fit
   * defines the line exactly but leaves no residual to spread. Distinct from
   * 0, which would claim a perfect measurement. */
  slope_stderr: number | null;
  intercept_stderr: number | null;
  r_squared: number;
  weighted: boolean;
  x_log: boolean;
  y_log: boolean;
};

/** Graph settings are stored as free-form JSON on the experiment row, so
 * nothing about their shape is guaranteed by the API. */
export type ExperimentConfig = Record<string, unknown>;

/**
 * Reads an axis label out of the free-form config.
 *
 * Anything that is not a string — absent, null, or a value written by an older
 * client — reads as the empty string, which callers treat as "fall back to the
 * default axis title".
 */
export function readAxisLabel(config: ExperimentConfig, key: string): string {
  const label = config[key];
  return typeof label === "string" ? label : "";
}

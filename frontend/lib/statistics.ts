export type ColumnStats = {
  n: number;
  mean: number;
  // Sample standard deviation (Bessel's correction, n-1 denominator) --
  // the standard estimator when the values are a sample of repeated
  // measurements rather than the full population. Undefined (null) for
  // fewer than 2 points, since variance has no meaning for a single value.
  stdev: number | null;
  // Standard error of the mean (stdev / sqrt(n)) -- the "uncertainty of
  // the mean" reported alongside it (mean ± sem), distinct from stdev
  // (the spread of individual measurements). Null under the same
  // n < 2 condition as stdev.
  sem: number | null;
};

export function computeColumnStats(values: number[]): ColumnStats {
  const n = values.length;
  const mean = values.reduce((sum, v) => sum + v, 0) / n;
  if (n < 2) {
    return { n, mean, stdev: null, sem: null };
  }
  const variance =
    values.reduce((sum, v) => sum + (v - mean) ** 2, 0) / (n - 1);
  const stdev = Math.sqrt(variance);
  const sem = stdev / Math.sqrt(n);
  return { n, mean, stdev, sem };
}

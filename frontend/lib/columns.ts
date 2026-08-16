// x and y always lead (matching the chart/editor convention), followed by
// any extra columns (e.g. y_error) in their original insertion order.
export function orderedColumnKeys(columns: Record<string, number[]>): string[] {
  return [
    "x",
    "y",
    ...Object.keys(columns).filter((key) => key !== "x" && key !== "y"),
  ].filter((key) => key in columns);
}

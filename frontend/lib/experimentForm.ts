/**
 * Parsing for the hidden `columns` field the experiment forms submit.
 *
 * The editors build the column map on the client and post it as JSON in a
 * hidden input, so the server action cannot assume it is well-formed: the
 * field can be missing entirely, hold something that is not JSON, or hold
 * valid JSON of the wrong shape.
 */

export type ParsedColumnsResult =
  | { ok: true; columns: Record<string, number[]> }
  | { ok: false; error: string };

export function parseColumnsField(
  value: FormDataEntryValue | null,
): ParsedColumnsResult {
  let columns: Record<string, number[]>;
  try {
    columns = JSON.parse(String(value ?? ""));
  } catch {
    return { ok: false, error: "データが正しく貼り付けられていません" };
  }
  if (!columns.x || !columns.y) {
    return { ok: false, error: "x列とy列のデータが必要です" };
  }
  return { ok: true, columns };
}

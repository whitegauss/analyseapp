/**
 * Parsing for the hidden `columns` field the experiment forms submit.
 *
 * The editors build the column map on the client and post it as JSON in a
 * hidden input, so the server action cannot assume it is well-formed: the
 * field can be missing entirely, hold something that is not JSON, or hold
 * valid JSON of the wrong shape. A form is also just an HTTP request, so any
 * of those can be sent deliberately rather than by accident.
 */

export type ParsedColumnsResult =
  | { ok: true; columns: Record<string, number[]> }
  | { ok: false; error: string };

// The messages reach the form as-is, so each one has to say what the user can
// go and change.
const PASTE_ERROR = "データが正しく貼り付けられていません";
const MISSING_XY_ERROR = "x列とy列のデータが必要です";
const UNEQUAL_LENGTH_ERROR = "すべての列の行数が揃っている必要があります";

/**
 * A column is a non-empty array of finite numbers, and nothing else.
 *
 * Checking the shape rather than truthiness is the whole point: `[]` and
 * `"invalid"` are both truthy, so a bare `!columns.x` test let an experiment
 * with no data through, and let a string sit inside a
 * `Record<string, number[]>` (KAN-61). JSON cannot spell NaN or Infinity, but
 * it can spell `1e999`, which parses to Infinity.
 */
function isDataColumn(value: unknown): value is number[] {
  return (
    Array.isArray(value) &&
    value.length > 0 &&
    value.every((n) => typeof n === "number" && Number.isFinite(n))
  );
}

export function parseColumnsField(
  value: FormDataEntryValue | null,
): ParsedColumnsResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(String(value ?? ""));
  } catch {
    return { ok: false, error: PASTE_ERROR };
  }

  // JSON.parse happily returns bare scalars and null. Reading `.x` off the
  // last of those used to throw a TypeError straight out of the server
  // action, so the user met Next.js's error page instead of this message.
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    return { ok: false, error: PASTE_ERROR };
  }

  const columns = parsed as Record<string, unknown>;
  if (!isDataColumn(columns.x) || !isDataColumn(columns.y)) {
    return { ok: false, error: MISSING_XY_ERROR };
  }

  // Extra columns (y_error, x_error, anything custom) go through the same
  // predicate. Skipping them would leave the return type lying about exactly
  // the columns nobody thought to check.
  const checked: Record<string, number[]> = {};
  for (const [name, column] of Object.entries(columns)) {
    if (!isDataColumn(column)) {
      return { ok: false, error: `${name}列に数値以外の値が含まれています` };
    }
    if (column.length !== columns.x.length) {
      return { ok: false, error: UNEQUAL_LENGTH_ERROR };
    }
    checked[name] = column;
  }

  return { ok: true, columns: checked };
}

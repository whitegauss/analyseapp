// Minimal RFC 4180 CSV field escaping: a field containing a comma, quote, or
// newline is wrapped in quotes with internal quotes doubled.
export function csvEscape(field: string): string {
  if (/[",\r\n]/.test(field)) {
    return `"${field.replace(/"/g, '""')}"`;
  }
  return field;
}

export function buildCsvRow(fields: (string | number)[]): string {
  return fields.map((field) => csvEscape(String(field))).join(",");
}

export type ParsedTable = {
  rows: string[][];
  columnCount: number;
  error?: string;
};

export function parsePastedText(text: string): ParsedTable {
  const lines = text
    .split("\n")
    .map((l) => l.trim())
    .filter((l) => l.length > 0);

  if (lines.length === 0) {
    return { rows: [], columnCount: 0 };
  }

  // Spreadsheet paste uses tabs; CSV uses commas; plain-text tables are
  // often separated by one or more spaces.
  const splitLine = lines.some((l) => l.includes("\t"))
    ? (l: string) => l.split("\t")
    : lines.some((l) => l.includes(","))
      ? (l: string) => l.split(",")
      : (l: string) => l.split(/\s+/);

  const rows = lines.map((l) => splitLine(l).map((c) => c.trim()));
  const columnCount = Math.max(...rows.map((r) => r.length));

  for (const row of rows) {
    if (row.length !== columnCount) {
      return {
        rows,
        columnCount,
        error: `列数が行によって異なります（${columnCount}列のはずが${row.length}列の行があります）`,
      };
    }
    for (const cell of row) {
      if (cell === "" || Number.isNaN(Number(cell))) {
        return {
          rows,
          columnCount,
          error: `数値として読めない値があります: "${cell}"`,
        };
      }
    }
  }

  return { rows, columnCount };
}

export function buildColumns(
  parsed: ParsedTable,
  extraRoles: Record<number, string>,
  customNames: Record<number, string>,
): Record<string, number[]> | null {
  if (parsed.error || parsed.rows.length === 0) return null;

  const result: Record<string, number[]> = { x: [], y: [] };
  const extraKeys: (string | null)[] = [];
  for (let col = 2; col < parsed.columnCount; col++) {
    const role = extraRoles[col] ?? "__ignore__";
    if (role === "__ignore__") {
      extraKeys.push(null);
    } else if (role === "__custom__") {
      const name = customNames[col]?.trim();
      extraKeys.push(name ? name : null);
    } else {
      extraKeys.push(role);
    }
  }
  extraKeys.forEach((key) => {
    if (key) result[key] = [];
  });

  for (const row of parsed.rows) {
    result.x.push(Number(row[0]));
    result.y.push(Number(row[1]));
    extraKeys.forEach((key, i) => {
      if (key) result[key].push(Number(row[i + 2]));
    });
  }

  return result;
}

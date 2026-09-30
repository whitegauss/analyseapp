"use client";

import { useMemo, useState } from "react";
import { parsePastedText } from "@/lib/pasteDataParsing";
import { splitColumns, summarizeColumn } from "@/lib/tools/measurementStats";

// Not tied to a saved experiment -- paste any list of numbers (one or more
// columns) and get per-column descriptive statistics, independent of
// /experiments/*. Reuses the same paste-parsing (tab/comma/space
// delimiter detection) as the raw-data entry forms, without the x/y role
// assignment those need (a column here is just "column N", not x/y/y_error).
export default function MeasurementStatsCalculator() {
  const [pastedText, setPastedText] = useState("");
  const parsed = useMemo(() => parsePastedText(pastedText), [pastedText]);

  const columns = useMemo(() => splitColumns(parsed), [parsed]);

  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        測定値をスプレッドシートなどからコピー＆ペーストしてください（複数列可、タブ／カンマ／スペース区切りを自動判定）。列ごとにn・平均±不確かさ（平均の標準誤差）・標準偏差を計算します。
      </p>
      <textarea
        value={pastedText}
        onChange={(e) => setPastedText(e.target.value)}
        rows={6}
        placeholder={"1.23\n1.25\n1.19\n1.31"}
        className="w-full rounded-md border border-zinc-300 px-2 py-1 font-mono text-sm dark:border-zinc-700 dark:bg-zinc-900"
      />

      {parsed.error && (
        <p className="text-sm text-red-600 dark:text-red-400">{parsed.error}</p>
      )}

      {columns.length > 0 && !parsed.error && (
        <div className="overflow-x-auto">
          <table className="w-full min-w-max text-left text-sm">
            <thead>
              <tr className="border-b border-zinc-200 text-xs text-zinc-500 dark:border-zinc-800 dark:text-zinc-400">
                <th className="py-1 pr-4 font-medium">列</th>
                <th className="py-1 pr-4 font-medium">n</th>
                <th className="py-1 pr-4 font-medium">平均 ± 不確かさ</th>
                <th className="py-1 pr-4 font-medium">標準偏差</th>
              </tr>
            </thead>
            <tbody>
              {columns.map((values, i) => {
                const { n, mean, stdev } = summarizeColumn(values);
                return (
                  <tr
                    key={i}
                    className="border-b border-zinc-100 last:border-0 dark:border-zinc-900"
                  >
                    <td className="py-1 pr-4 text-zinc-900 dark:text-zinc-50">
                      列{i + 1}
                    </td>
                    <td className="py-1 pr-4 text-zinc-700 dark:text-zinc-300">
                      {n}
                    </td>
                    <td className="py-1 pr-4 text-zinc-700 dark:text-zinc-300">
                      {mean}
                    </td>
                    <td className="py-1 pr-4 text-zinc-700 dark:text-zinc-300">
                      {stdev}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

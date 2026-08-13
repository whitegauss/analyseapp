"use client";

import { useMemo, useState } from "react";
import { useActionState } from "react";
import {
  createExperiment,
  type CreateExperimentState,
} from "@/app/experiments/actions";
import ExperimentChart from "./ExperimentChart";
import ChartSkeleton from "./ChartSkeleton";
import AxisLabelRuns, { type AxisLabelRun } from "./AxisLabelRuns";

function AxisLabelRunsEditor({
  label,
  runs,
  onChange,
}: {
  label: string;
  runs: AxisLabelRun[];
  onChange: (runs: AxisLabelRun[]) => void;
}) {
  const update = (i: number, patch: Partial<AxisLabelRun>) =>
    onChange(runs.map((r, idx) => (idx === i ? { ...r, ...patch } : r)));
  const remove = (i: number) => onChange(runs.filter((_, idx) => idx !== i));
  const add = () => onChange([...runs, { text: "", italic: true }]);

  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-sm font-medium text-zinc-700 dark:text-zinc-300">
        {label}
      </span>
      {runs.map((run, i) => (
        <div key={i} className="flex items-center gap-2">
          <input
            type="text"
            value={run.text}
            onChange={(e) => update(i, { text: e.target.value })}
            placeholder="例: v, m/s, （速度）"
            className="flex-1 rounded-md border border-zinc-300 bg-transparent px-2 py-1 text-sm dark:border-zinc-700"
          />
          <label className="flex items-center gap-1 text-xs text-zinc-600 dark:text-zinc-400">
            <input
              type="checkbox"
              checked={run.italic}
              onChange={(e) => update(i, { italic: e.target.checked })}
            />
            斜体（変数）
          </label>
          <button
            type="button"
            onClick={() => remove(i)}
            className="text-xs text-red-600 dark:text-red-400"
          >
            削除
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={add}
        className="self-start text-xs text-zinc-600 underline dark:text-zinc-400"
      >
        + 断片を追加
      </button>
      {runs.length > 0 && (
        <div className="rounded-md border border-zinc-200 px-3 py-2 text-sm dark:border-zinc-800">
          プレビュー: <AxisLabelRuns runs={runs} />
        </div>
      )}
    </div>
  );
}

const ROLE_OPTIONS = [
  { value: "y_error", label: "y誤差 (y_error)" },
  { value: "x_error", label: "x誤差 (x_error)" },
  { value: "__ignore__", label: "使わない" },
  { value: "__custom__", label: "その他（名前を指定）" },
] as const;

type ParsedTable = {
  rows: string[][];
  columnCount: number;
  error?: string;
};

function parsePastedText(text: string): ParsedTable {
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
        return { rows, columnCount, error: `数値として読めない値があります: "${cell}"` };
      }
    }
  }

  return { rows, columnCount };
}

const initialState: CreateExperimentState = {};

export default function ExperimentEditor() {
  const [state, formAction, pending] = useActionState(
    createExperiment,
    initialState,
  );
  const [title, setTitle] = useState("");
  const [pastedText, setPastedText] = useState("");
  const [extraRoles, setExtraRoles] = useState<Record<number, string>>({});
  const [customNames, setCustomNames] = useState<Record<number, string>>({});
  const [xAxisRuns, setXAxisRuns] = useState<AxisLabelRun[]>([]);
  const [yAxisRuns, setYAxisRuns] = useState<AxisLabelRun[]>([]);

  const parsed = useMemo(() => parsePastedText(pastedText), [pastedText]);

  const columns = useMemo(() => {
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
  }, [parsed, extraRoles, customNames]);

  const extraColumnIndexes = Array.from(
    { length: Math.max(parsed.columnCount - 2, 0) },
    (_, i) => i + 2,
  );

  return (
    <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
      <form action={formAction} className="flex flex-col gap-4">
        <label className="flex flex-col gap-1 text-sm text-zinc-700 dark:text-zinc-300">
          タイトル
          <input
            type="text"
            name="title"
            required
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            className="rounded-md border border-zinc-300 bg-transparent px-3 py-2 text-sm dark:border-zinc-700"
          />
        </label>

        <label className="flex flex-col gap-1 text-sm text-zinc-700 dark:text-zinc-300">
          データ（スプレッドシートからコピー＆ペースト。1列目=x, 2列目=y）
          <textarea
            rows={8}
            value={pastedText}
            onChange={(e) => setPastedText(e.target.value)}
            placeholder={"0\t1\n1\t3\n2\t5"}
            className="rounded-md border border-zinc-300 bg-transparent px-3 py-2 font-mono text-sm dark:border-zinc-700"
          />
        </label>

        {parsed.error && (
          <p className="text-sm text-red-600 dark:text-red-400">{parsed.error}</p>
        )}

        {extraColumnIndexes.length > 0 && !parsed.error && (
          <div className="flex flex-col gap-2">
            <h2 className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
              3列目以降の役割
            </h2>
            {extraColumnIndexes.map((col) => (
              <div key={col} className="flex items-center gap-2">
                <span className="text-sm text-zinc-600 dark:text-zinc-400">
                  {col + 1}列目:
                </span>
                <select
                  value={extraRoles[col] ?? "__ignore__"}
                  onChange={(e) =>
                    setExtraRoles((prev) => ({ ...prev, [col]: e.target.value }))
                  }
                  className="rounded-md border border-zinc-300 bg-transparent px-2 py-1 text-sm dark:border-zinc-700"
                >
                  {ROLE_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
                {extraRoles[col] === "__custom__" && (
                  <input
                    type="text"
                    placeholder="カラム名"
                    value={customNames[col] ?? ""}
                    onChange={(e) =>
                      setCustomNames((prev) => ({ ...prev, [col]: e.target.value }))
                    }
                    className="rounded-md border border-zinc-300 bg-transparent px-2 py-1 text-sm dark:border-zinc-700"
                  />
                )}
              </div>
            ))}
          </div>
        )}

        {columns && (
          <div className="flex flex-col gap-4 rounded-md border border-zinc-200 p-3 dark:border-zinc-800">
            <h2 className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
              軸ラベル（任意。断片ごとに斜体＝変数／立体＝単位・日本語などを指定）
            </h2>
            <AxisLabelRunsEditor label="X軸" runs={xAxisRuns} onChange={setXAxisRuns} />
            <AxisLabelRunsEditor label="Y軸" runs={yAxisRuns} onChange={setYAxisRuns} />
          </div>
        )}

        {columns && (
          <div className="flex flex-col gap-2">
            <h2 className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
              プレビュー（{parsed.rows.length}行）
            </h2>
            <div className="overflow-x-auto rounded-md border border-zinc-200 dark:border-zinc-800">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-zinc-200 dark:border-zinc-800">
                    {Object.keys(columns).map((key) => (
                      <th key={key} className="px-3 py-1.5 text-left font-medium">
                        {key}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {columns.x.slice(0, 5).map((_, i) => (
                    <tr key={i}>
                      {Object.keys(columns).map((key) => (
                        <td key={key} className="px-3 py-1">
                          {columns[key][i]}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
              {columns.x.length > 5 && (
                <p className="px-3 py-1 text-xs text-zinc-400">
                  他 {columns.x.length - 5} 行...
                </p>
              )}
            </div>
          </div>
        )}

        <input type="hidden" name="columns" value={columns ? JSON.stringify(columns) : ""} />
        <input type="hidden" name="xAxisLabelRuns" value={JSON.stringify(xAxisRuns)} />
        <input type="hidden" name="yAxisLabelRuns" value={JSON.stringify(yAxisRuns)} />

        {state.error && (
          <p className="text-sm text-red-600 dark:text-red-400">{state.error}</p>
        )}

        <button
          type="submit"
          disabled={pending || !columns}
          className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-zinc-50 dark:text-zinc-900"
        >
          {pending ? "保存中..." : "保存してグラフを確定"}
        </button>
      </form>

      <div className="flex flex-col gap-2">
        <h2 className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
          プレビュー（未保存）
        </h2>
        {columns ? (
          <ExperimentChart
            columns={columns}
            title={title || "(タイトル未入力)"}
            xAxisLabelRuns={xAxisRuns}
            yAxisLabelRuns={yAxisRuns}
          />
        ) : (
          <ChartSkeleton />
        )}
      </div>
    </div>
  );
}

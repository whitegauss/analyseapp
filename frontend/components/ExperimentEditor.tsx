"use client";

import { useMemo, useState } from "react";
import { useActionState } from "react";
import {
  createExperiment,
  type CreateExperimentState,
} from "@/app/experiments/actions";
import ExperimentChart from "./ExperimentChart";
import ChartSkeleton from "./ChartSkeleton";

function AxisLabelInput({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="flex flex-col gap-1.5 text-sm text-zinc-700 dark:text-zinc-300">
      {label}
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="例: 速度 $v$ (m/s)"
        className="rounded-md border border-zinc-300 bg-transparent px-2 py-1.5 text-sm dark:border-zinc-700"
      />
    </label>
  );
}

function InfoTooltip({ text }: { text: string }) {
  return (
    <span className="group relative inline-flex cursor-help items-center">
      <span className="flex h-4 w-4 items-center justify-center rounded-full border border-zinc-400 text-[10px] leading-none text-zinc-500 dark:border-zinc-600 dark:text-zinc-400">
        i
      </span>
      <span className="pointer-events-none absolute bottom-full left-1/2 z-10 mb-1 w-64 -translate-x-1/2 rounded-md border border-zinc-200 bg-white p-2 text-xs font-normal text-zinc-700 opacity-0 shadow-lg transition-opacity group-hover:opacity-100 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-200">
        {text}
      </span>
    </span>
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
  const [xAxisLabel, setXAxisLabel] = useState("");
  const [yAxisLabel, setYAxisLabel] = useState("");

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
          タイトル（任意）
          <input
            type="text"
            name="title"
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

        <div className="flex flex-col gap-4 rounded-md border border-zinc-200 p-3 dark:border-zinc-800">
          <h2 className="flex items-center gap-1.5 text-sm font-medium text-zinc-500 dark:text-zinc-400">
            軸ラベル
            <InfoTooltip text="任意入力です。$...$で囲んだ部分だけTeXの数式として斜体表示、それ以外は日本語も含めそのまま立体表示されます（例: 速度 $v$ (m/s)）" />
          </h2>
          <AxisLabelInput label="X軸" value={xAxisLabel} onChange={setXAxisLabel} />
          <AxisLabelInput label="Y軸" value={yAxisLabel} onChange={setYAxisLabel} />
        </div>

        <input type="hidden" name="columns" value={columns ? JSON.stringify(columns) : ""} />
        <input type="hidden" name="xAxisLabel" value={xAxisLabel} />
        <input type="hidden" name="yAxisLabel" value={yAxisLabel} />

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
            xAxisLabel={xAxisLabel}
            yAxisLabel={yAxisLabel}
          />
        ) : (
          <ChartSkeleton />
        )}
      </div>
    </div>
  );
}

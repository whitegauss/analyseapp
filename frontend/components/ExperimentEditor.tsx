"use client";

import { useMemo, useState } from "react";
import { useActionState } from "react";
import {
  createExperiment,
  type CreateExperimentState,
} from "@/app/experiments/actions";
import { parsePastedText, buildColumns } from "@/lib/pasteDataParsing";
import ExperimentChart from "./ExperimentChart";
import ChartSkeleton from "./ChartSkeleton";
import ColumnRoleSelector from "./ColumnRoleSelector";
import AxisLabelInput from "./AxisLabelInput";
import InfoTooltip from "./InfoTooltip";

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
  const columns = useMemo(
    () => buildColumns(parsed, extraRoles, customNames),
    [parsed, extraRoles, customNames],
  );

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
          <p className="text-sm text-red-600 dark:text-red-400">
            {parsed.error}
          </p>
        )}

        {extraColumnIndexes.length > 0 && !parsed.error && (
          <ColumnRoleSelector
            extraColumnIndexes={extraColumnIndexes}
            extraRoles={extraRoles}
            customNames={customNames}
            onRoleChange={(col, value) =>
              setExtraRoles((prev) => ({ ...prev, [col]: value }))
            }
            onCustomNameChange={(col, value) =>
              setCustomNames((prev) => ({ ...prev, [col]: value }))
            }
          />
        )}

        <div className="flex flex-col gap-4 rounded-md border border-zinc-200 p-3 dark:border-zinc-800">
          <h2 className="flex items-center gap-1.5 text-sm font-medium text-zinc-500 dark:text-zinc-400">
            軸ラベル
            <InfoTooltip text="任意入力です。$...$で囲んだ部分だけTeXの数式として斜体表示、それ以外は日本語も含めそのまま立体表示されます（例: 速度 $v$ (m/s)）" />
          </h2>
          <AxisLabelInput
            label="X軸"
            value={xAxisLabel}
            onChange={setXAxisLabel}
          />
          <AxisLabelInput
            label="Y軸"
            value={yAxisLabel}
            onChange={setYAxisLabel}
          />
        </div>

        <input
          type="hidden"
          name="columns"
          value={columns ? JSON.stringify(columns) : ""}
        />
        <input type="hidden" name="xAxisLabel" value={xAxisLabel} />
        <input type="hidden" name="yAxisLabel" value={yAxisLabel} />

        {state.error && (
          <p className="text-sm text-red-600 dark:text-red-400">
            {state.error}
          </p>
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

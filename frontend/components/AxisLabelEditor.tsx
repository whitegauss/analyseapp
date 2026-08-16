"use client";

import { useActionState, useState } from "react";
import {
  updateAxisLabels,
  type UpdateAxisLabelsState,
} from "@/app/experiments/actions";
import AxisLabelInput from "./AxisLabelInput";
import InfoTooltip from "./InfoTooltip";

const initialState: UpdateAxisLabelsState = {};

type Props = {
  id: string;
  xAxisLabel: string;
  yAxisLabel: string;
};

export default function AxisLabelEditor({ id, xAxisLabel, yAxisLabel }: Props) {
  const [editing, setEditing] = useState(false);
  const [state, formAction, pending] = useActionState(
    updateAxisLabels,
    initialState,
  );
  const [x, setX] = useState(xAxisLabel);
  const [y, setY] = useState(yAxisLabel);

  if (!editing) {
    return (
      <button
        type="button"
        onClick={() => setEditing(true)}
        className="self-start text-xs text-zinc-500 underline hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
      >
        軸ラベルを編集
      </button>
    );
  }

  return (
    <form
      action={formAction}
      className="flex w-full flex-col gap-3 rounded-md border border-zinc-200 p-3 dark:border-zinc-800"
    >
      <div className="flex items-center gap-1.5 text-sm font-medium text-zinc-500 dark:text-zinc-400">
        軸ラベルを編集
        <InfoTooltip text="任意入力です。$...$で囲んだ部分だけTeXの数式として斜体表示、それ以外は日本語も含めそのまま立体表示されます（例: 速度 $v$ (m/s)）" />
      </div>
      <input type="hidden" name="id" value={id} />
      <AxisLabelInput label="X軸" value={x} onChange={setX} />
      <AxisLabelInput label="Y軸" value={y} onChange={setY} />
      <input type="hidden" name="xAxisLabel" value={x} />
      <input type="hidden" name="yAxisLabel" value={y} />
      {state.error && (
        <p className="text-sm text-red-600 dark:text-red-400">{state.error}</p>
      )}
      <div className="flex items-center gap-2">
        <button
          type="submit"
          disabled={pending}
          className="rounded-md bg-zinc-900 px-3 py-1.5 text-xs font-medium text-white disabled:opacity-50 dark:bg-zinc-50 dark:text-zinc-900"
        >
          {pending ? "保存中..." : "保存"}
        </button>
        <button
          type="button"
          onClick={() => setEditing(false)}
          className="text-xs text-zinc-500 underline hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
        >
          キャンセル
        </button>
      </div>
    </form>
  );
}

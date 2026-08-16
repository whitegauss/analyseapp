"use client";

import { useActionState, useState } from "react";
import {
  updateAxisLabels,
  type UpdateAxisLabelsState,
} from "@/app/experiments/actions";
import AxisLabelInput from "./AxisLabelInput";
import InfoTooltip from "./InfoTooltip";
import InlineEditCard from "./InlineEditCard";

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

  return (
    <InlineEditCard
      label="軸ラベルを編集"
      editing={editing}
      onStartEditing={() => setEditing(true)}
      onCancel={() => setEditing(false)}
      formAction={formAction}
      pending={pending}
      error={state.error}
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
    </InlineEditCard>
  );
}

"use client";

import { useState } from "react";
import {
  formatUncertainty,
  roundToUncertainty,
} from "@/lib/significantFigures";

const inputClass =
  "w-40 rounded-md border border-zinc-300 px-2 py-1 dark:border-zinc-700 dark:bg-zinc-900";

export default function SignificantFigureRounder() {
  const [value, setValue] = useState("");
  const [uncertainty, setUncertainty] = useState("");

  const parsedValue = Number(value);
  const parsedUncertainty = Number(uncertainty);
  const valid =
    value !== "" &&
    uncertainty !== "" &&
    Number.isFinite(parsedValue) &&
    Number.isFinite(parsedUncertainty);

  const result = valid
    ? (() => {
        const { rounded, decimals } = roundToUncertainty(
          parsedValue,
          parsedUncertainty,
        );
        return `${rounded.toFixed(decimals)} ± ${formatUncertainty(parsedUncertainty)}`;
      })()
    : null;

  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        値と1σ不確かさを入力すると、グラフの回帰直線表示と同じ規約（不確かさの先頭有効数字に合わせて丸め、小数点以下は最大4桁まで）で表示を計算します。
      </p>
      <div className="flex flex-wrap gap-3">
        <label className="flex flex-col gap-1 text-sm">
          値
          <input
            type="number"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            className={inputClass}
          />
        </label>
        <label className="flex flex-col gap-1 text-sm">
          不確かさ（1σ）
          <input
            type="number"
            value={uncertainty}
            onChange={(e) => setUncertainty(e.target.value)}
            className={inputClass}
          />
        </label>
      </div>
      {result && (
        <p className="text-lg font-medium text-zinc-900 dark:text-zinc-50">
          {result}
        </p>
      )}
    </div>
  );
}

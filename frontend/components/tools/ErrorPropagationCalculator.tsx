"use client";

import { useMemo, useState } from "react";
import {
  propagateError,
  type PropagationOperation,
} from "@/lib/errorPropagation";
import {
  formatUncertainty,
  roundToUncertainty,
} from "@/lib/significantFigures";

const OPERATIONS: { id: PropagationOperation; label: string }[] = [
  { id: "add", label: "加算  z = x + y" },
  { id: "subtract", label: "減算  z = x - y" },
  { id: "multiply", label: "乗算  z = x × y" },
  { id: "divide", label: "除算  z = x / y" },
  { id: "power", label: "べき乗  z = xⁿ（nは不確かさ無しの定数）" },
];

const inputClass =
  "w-32 rounded-md border border-zinc-300 px-2 py-1 dark:border-zinc-700 dark:bg-zinc-900";

export default function ErrorPropagationCalculator() {
  const [operation, setOperation] = useState<PropagationOperation>("add");
  const [x, setX] = useState("");
  const [sx, setSx] = useState("");
  const [y, setY] = useState(""); // also doubles as the exponent n for "power"
  const [sy, setSy] = useState("");

  const isPower = operation === "power";

  const result = useMemo(() => {
    const parsedX = Number(x);
    const parsedSx = Number(sx);
    const parsedY = Number(y);
    const parsedSy = isPower ? 0 : Number(sy);

    const valid =
      x !== "" &&
      sx !== "" &&
      y !== "" &&
      (isPower || sy !== "") &&
      Number.isFinite(parsedX) &&
      Number.isFinite(parsedSx) &&
      Number.isFinite(parsedY) &&
      Number.isFinite(parsedSy);
    if (!valid) return null;

    const propagated = propagateError(
      operation,
      parsedX,
      parsedSx,
      parsedY,
      parsedSy,
    );
    if (!Number.isFinite(propagated.value)) return null;

    const { rounded, decimals } = roundToUncertainty(
      propagated.value,
      propagated.uncertainty,
    );
    return `${rounded.toFixed(decimals)} ± ${formatUncertainty(propagated.uncertainty)}`;
  }, [operation, x, sx, y, sy, isPower]);

  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-zinc-600 dark:text-zinc-400">
        x,
        yは互いに独立で相関が無い測定値として、線形近似（1次のテイラー展開）による誤差伝播を計算します。
      </p>
      <label className="flex flex-col gap-1 text-sm">
        演算
        <select
          value={operation}
          onChange={(e) => setOperation(e.target.value as PropagationOperation)}
          className="w-fit rounded-md border border-zinc-300 px-2 py-1 dark:border-zinc-700 dark:bg-zinc-900"
        >
          {OPERATIONS.map((op) => (
            <option key={op.id} value={op.id}>
              {op.label}
            </option>
          ))}
        </select>
      </label>

      <div className="flex flex-wrap gap-3">
        <label className="flex flex-col gap-1 text-sm">
          x
          <input
            type="number"
            value={x}
            onChange={(e) => setX(e.target.value)}
            className={inputClass}
          />
        </label>
        <label className="flex flex-col gap-1 text-sm">
          σx（xの1σ不確かさ）
          <input
            type="number"
            value={sx}
            onChange={(e) => setSx(e.target.value)}
            className={inputClass}
          />
        </label>
        <label className="flex flex-col gap-1 text-sm">
          {isPower ? "n（指数、定数）" : "y"}
          <input
            type="number"
            value={y}
            onChange={(e) => setY(e.target.value)}
            className={inputClass}
          />
        </label>
        {!isPower && (
          <label className="flex flex-col gap-1 text-sm">
            σy（yの1σ不確かさ）
            <input
              type="number"
              value={sy}
              onChange={(e) => setSy(e.target.value)}
              className={inputClass}
            />
          </label>
        )}
      </div>

      {result ? (
        <p className="text-lg font-medium text-zinc-900 dark:text-zinc-50">
          z = {result}
        </p>
      ) : (
        x !== "" &&
        sx !== "" &&
        y !== "" && (
          <p className="text-sm text-red-600 dark:text-red-400">
            この入力では計算できません（除算でy=0など）
          </p>
        )
      )}
    </div>
  );
}

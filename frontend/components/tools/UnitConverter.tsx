"use client";

import { useState } from "react";
import { convertUnit, unitCategories } from "@/lib/unitConversion";

const selectClass =
  "rounded-md border border-zinc-300 px-2 py-1 dark:border-zinc-700 dark:bg-zinc-900";

export default function UnitConverter() {
  const [categoryId, setCategoryId] = useState(unitCategories[0].id);
  const category =
    unitCategories.find((c) => c.id === categoryId) ?? unitCategories[0];
  const [fromUnitId, setFromUnitId] = useState(category.units[0].id);
  const [toUnitId, setToUnitId] = useState(
    category.units[1]?.id ?? category.units[0].id,
  );
  const [value, setValue] = useState("");

  function handleCategoryChange(id: string) {
    const next = unitCategories.find((c) => c.id === id);
    if (!next) return;
    setCategoryId(id);
    setFromUnitId(next.units[0].id);
    setToUnitId(next.units[1]?.id ?? next.units[0].id);
  }

  const parsedValue = Number(value);
  const result =
    value !== "" && Number.isFinite(parsedValue)
      ? convertUnit(categoryId, fromUnitId, toUnitId, parsedValue)
      : null;

  const fromLabel = category.units.find((u) => u.id === fromUnitId)?.label;
  const toLabel = category.units.find((u) => u.id === toUnitId)?.label;

  return (
    <div className="flex flex-col gap-3">
      <label className="flex flex-col gap-1 text-sm">
        種類
        <select
          value={categoryId}
          onChange={(e) => handleCategoryChange(e.target.value)}
          className={`w-fit ${selectClass}`}
        >
          {unitCategories.map((c) => (
            <option key={c.id} value={c.id}>
              {c.label}
            </option>
          ))}
        </select>
      </label>

      <div className="flex flex-wrap items-end gap-3">
        <label className="flex flex-col gap-1 text-sm">
          値
          <input
            type="number"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            className="w-32 rounded-md border border-zinc-300 px-2 py-1 dark:border-zinc-700 dark:bg-zinc-900"
          />
        </label>
        <label className="flex flex-col gap-1 text-sm">
          変換元
          <select
            value={fromUnitId}
            onChange={(e) => setFromUnitId(e.target.value)}
            className={selectClass}
          >
            {category.units.map((u) => (
              <option key={u.id} value={u.id}>
                {u.label}
              </option>
            ))}
          </select>
        </label>
        <span className="pb-2 text-zinc-400">→</span>
        <label className="flex flex-col gap-1 text-sm">
          変換先
          <select
            value={toUnitId}
            onChange={(e) => setToUnitId(e.target.value)}
            className={selectClass}
          >
            {category.units.map((u) => (
              <option key={u.id} value={u.id}>
                {u.label}
              </option>
            ))}
          </select>
        </label>
      </div>

      {result !== null && (
        <p className="text-lg font-medium text-zinc-900 dark:text-zinc-50">
          {value} {fromLabel} = {Number(result.toPrecision(6))} {toLabel}
        </p>
      )}
    </div>
  );
}

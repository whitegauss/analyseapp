"use client";

import { useMemo, useState } from "react";
import {
  propagate,
  type MeasuredValue,
  type PropagationResult,
} from "@/lib/errorPropagation";
import {
  constantNames,
  FormulaError,
  functionNames,
  parseFormula,
  type ParsedFormula,
} from "@/lib/formula";
import {
  formatToPrecision,
  formatUncertainty,
  roundToUncertainty,
} from "@/lib/significantFigures";

const EXAMPLES = [
  { label: "単振り子の周期", formula: "2*pi*sqrt(L/g)" },
  { label: "運動エネルギー", formula: "1/2*m*v^2" },
  { label: "位置エネルギー", formula: "m*g*h" },
  { label: "重力加速度（振り子から）", formula: "4*pi^2*L/T^2" },
  { label: "抵抗率", formula: "R*pi*d^2/(4*L)" },
  { label: "スネルの法則", formula: "sin(i)/sin(r)" },
];

const inputClass =
  "w-28 rounded-md border border-zinc-300 px-2 py-1 text-right tabular-nums dark:border-zinc-700 dark:bg-zinc-900";

// Entered text is kept per variable name rather than per row, so editing the
// formula (adding a term, fixing a typo) keeps the numbers already typed for
// every name that survives the edit.
type Entry = { value: string; uncertainty: string };

export default function ErrorPropagationCalculator() {
  const [source, setSource] = useState("2*pi*sqrt(L/g)");
  const [entries, setEntries] = useState<Record<string, Entry>>({});

  const parsed = useMemo((): {
    formula: ParsedFormula | null;
    error: string | null;
  } => {
    if (source.trim() === "") return { formula: null, error: null };
    try {
      return { formula: parseFormula(source), error: null };
    } catch (e) {
      if (e instanceof FormulaError) return { formula: null, error: e.message };
      throw e;
    }
  }, [source]);

  // Memoised so the identity is stable while the formula is unchanged: the
  // propagation below depends on it, and a fresh [] each render would redo
  // the work on every keystroke anywhere on the page.
  const variables = useMemo(
    () => parsed.formula?.variables ?? [],
    [parsed.formula],
  );

  const result = useMemo((): PropagationResult | null => {
    if (!parsed.formula || variables.length === 0) return null;

    const measured: Record<string, MeasuredValue> = Object.create(null);
    for (const name of variables) {
      const entry = entryOf(entries, name);
      if (!entry || entry.value.trim() === "") return null;
      const value = Number(entry.value);
      // A blank uncertainty means "exact", which is how constants written
      // into the formula as a name (g, c, ...) are meant to behave.
      const uncertainty =
        entry.uncertainty.trim() === "" ? 0 : Number(entry.uncertainty);
      if (!Number.isFinite(value) || !Number.isFinite(uncertainty)) return null;
      measured[name] = { value, uncertainty: Math.abs(uncertainty) };
    }

    try {
      const propagated = propagate(parsed.formula, measured);
      // The uncertainty is checked as well as the value: sqrt(x) at x = 0 has
      // a finite value and an infinite derivative, and formatUncertainty
      // renders a non-finite uncertainty as "0.0" -- an exact-looking answer
      // to a formula that is anything but.
      if (
        !Number.isFinite(propagated.value) ||
        !Number.isFinite(propagated.uncertainty)
      ) {
        return null;
      }
      return propagated;
    } catch {
      return null;
    }
  }, [parsed.formula, variables, entries]);

  const allFilled =
    variables.length > 0 &&
    variables.every(
      (name) => (entryOf(entries, name)?.value ?? "").trim() !== "",
    );

  const setEntry = (name: string, patch: Partial<Entry>) =>
    setEntries((current) => {
      const next: Record<string, Entry> = Object.create(null);
      Object.assign(next, current);
      const existing = entryOf(current, name);
      next[name] = {
        value: existing?.value ?? "",
        uncertainty: existing?.uncertainty ?? "",
        ...patch,
      };
      return next;
    });

  const { rounded, decimals } = roundToUncertainty(
    result?.value ?? 0,
    result?.uncertainty ?? null,
  );

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1 text-sm text-zinc-600 dark:text-zinc-400">
        <p>
          式を書くと、出てきた文字が測定量として下に並びます。各変数の値と1σ不確かさを入れると、偏微分（自動微分で厳密に計算）を使った線形近似の誤差伝播
          σz = √Σ(∂z/∂xi・σxi)² を計算します。
        </p>
        <p>
          各変数は互いに独立で相関が無いものとして扱います（相関がある場合の共分散項は含みません）。
        </p>
      </div>

      <label className="flex flex-col gap-1 text-sm">
        式
        <input
          type="text"
          value={source}
          onChange={(e) => setSource(e.target.value)}
          spellCheck={false}
          placeholder="例: 2*pi*sqrt(L/g)"
          className="w-full rounded-md border border-zinc-300 px-3 py-2 font-mono dark:border-zinc-700 dark:bg-zinc-900"
        />
      </label>

      <div className="flex flex-wrap items-center gap-2 text-xs">
        <span className="text-zinc-500 dark:text-zinc-400">例:</span>
        {EXAMPLES.map((example) => (
          <button
            key={example.formula}
            type="button"
            onClick={() => setSource(example.formula)}
            className="rounded-full border border-zinc-300 px-2 py-1 text-zinc-700 hover:bg-zinc-100 dark:border-zinc-700 dark:text-zinc-300 dark:hover:bg-zinc-800"
          >
            {example.label}
          </button>
        ))}
      </div>

      <details className="text-xs text-zinc-600 dark:text-zinc-400">
        <summary className="cursor-pointer">使える書き方</summary>
        <ul className="mt-2 flex list-disc flex-col gap-1 pl-5">
          <li>
            演算子: <code>+ - * / ^</code>（掛け算の <code>*</code>{" "}
            は省略できません）と括弧
          </li>
          <li>関数: {functionNames.join(", ")}</li>
          <li>定数: {constantNames.join(", ")}（変数としては使えません）</li>
          <li>
            三角関数の引数はラジアンです。度で入れたいときは{" "}
            <code>sin(theta*pi/180)</code> と書いてください
          </li>
          <li>
            変数名にはギリシャ文字も使えます（<code>θ</code>・<code>λ</code>）。
            <code>1.6e-19</code> のような指数表記も書けます
          </li>
          <li>σ を空欄にした変数は「厳密な定数」として扱います</li>
        </ul>
      </details>

      {parsed.error && (
        <p className="text-sm text-red-600 dark:text-red-400">{parsed.error}</p>
      )}

      {variables.length > 0 && (
        <table className="w-full max-w-2xl text-sm">
          <thead className="text-left text-zinc-500 dark:text-zinc-400">
            <tr>
              <th className="py-1 pr-3 font-medium">変数</th>
              <th className="py-1 pr-3 font-medium">値</th>
              <th className="py-1 pr-3 font-medium">σ（1σ不確かさ）</th>
              <th className="py-1 pr-3 font-medium">∂z/∂x</th>
              <th className="py-1 font-medium">誤差への寄与</th>
            </tr>
          </thead>
          <tbody>
            {variables.map((name, index) => {
              const term = result?.terms[index];
              return (
                <tr
                  key={name}
                  className="border-t border-zinc-200 dark:border-zinc-800"
                >
                  <td className="py-1 pr-3 font-mono">{name}</td>
                  <td className="py-1 pr-3">
                    <input
                      type="number"
                      step="any"
                      value={entryOf(entries, name)?.value ?? ""}
                      onChange={(e) =>
                        setEntry(name, { value: e.target.value })
                      }
                      className={inputClass}
                    />
                  </td>
                  <td className="py-1 pr-3">
                    <input
                      type="number"
                      step="any"
                      value={entryOf(entries, name)?.uncertainty ?? ""}
                      onChange={(e) =>
                        setEntry(name, { uncertainty: e.target.value })
                      }
                      placeholder="0（厳密）"
                      className={inputClass}
                    />
                  </td>
                  <td className="py-1 pr-3 tabular-nums text-zinc-600 dark:text-zinc-400">
                    {term ? formatToPrecision(term.partial) : "—"}
                  </td>
                  <td className="py-1">
                    {term ? (
                      <div className="flex items-center gap-2">
                        <span
                          aria-hidden
                          className="h-2 w-24 overflow-hidden rounded-full bg-zinc-200 dark:bg-zinc-800"
                        >
                          <span
                            className="block h-full rounded-full bg-zinc-500 dark:bg-zinc-400"
                            style={{
                              width: `${Math.round(term.share * 100)}%`,
                            }}
                          />
                        </span>
                        <span className="tabular-nums text-zinc-600 dark:text-zinc-400">
                          {(term.share * 100).toFixed(0)}%
                        </span>
                      </div>
                    ) : (
                      <span className="text-zinc-400 dark:text-zinc-500">
                        —
                      </span>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}

      {result ? (
        <div className="flex flex-col gap-1">
          <p className="text-lg font-medium text-zinc-900 dark:text-zinc-50">
            z = {rounded.toFixed(decimals)} ±{" "}
            {formatUncertainty(result.uncertainty)}
          </p>
          {result.value !== 0 && result.uncertainty > 0 && (
            <p className="text-sm text-zinc-600 dark:text-zinc-400">
              相対不確かさ{" "}
              {((result.uncertainty / Math.abs(result.value)) * 100).toFixed(2)}
              %
            </p>
          )}
        </div>
      ) : (
        allFilled &&
        !parsed.error && (
          <p className="text-sm text-red-600 dark:text-red-400">
            この入力では計算できません（0除算・負の数の平方根・定義域外など）
          </p>
        )
      )}
    </div>
  );
}

// Variable names come from the formula the user typed, so a plain property
// read would resolve `constructor` or `toString` to something inherited --
// and `entry.value.trim()` on a function is a crash, not a wrong number.
function entryOf(
  entries: Record<string, Entry>,
  name: string,
): Entry | undefined {
  return Object.hasOwn(entries, name) ? entries[name] : undefined;
}

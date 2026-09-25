"use client";

import { useActionState, useMemo, useState } from "react";
import { updateFit, type UpdateFitState } from "@/app/experiments/actions";
import { fitParameters, type FitConfig } from "@/lib/fit";
import InfoTooltip from "./InfoTooltip";
import InlineEditCard from "./InlineEditCard";

const initialState: UpdateFitState = {};

type Props = {
  id: string;
  fit: FitConfig | null;
};

// Chooses the model the saved chart fits (KAN-29): the default straight line,
// or a formula the user types. Saving reloads the page, which runs the fit on
// the server the same way the straight line is.
export default function FitEditor({ id, fit }: Props) {
  const [editing, setEditing] = useState(false);
  const [state, formAction, pending] = useActionState(updateFit, initialState);
  const [mode, setMode] = useState<"linear" | "curve">(
    fit ? "curve" : "linear",
  );
  const [formula, setFormula] = useState(fit?.formula ?? "");
  // Kept as the text typed, not as numbers, so a half-typed "-" or "1e"
  // survives re-renders; converted only when building the hidden field.
  const [initialText, setInitialText] = useState<Record<string, string>>(() =>
    Object.fromEntries(
      Object.entries(fit?.initial ?? {}).map(([k, v]) => [k, String(v)]),
    ),
  );

  const params = useMemo(() => fitParameters(formula), [formula]);

  // Only the parameters the current formula has are sent; a starting value
  // typed for a parameter since deleted from the formula is dropped rather
  // than rejected by the server.
  const initial = useMemo(() => {
    const values: Record<string, number> = {};
    const invalid: string[] = [];
    if (params.ok) {
      for (const name of params.names) {
        const text = (initialText[name] ?? "").trim();
        if (text === "") continue;
        const value = Number(text);
        if (Number.isFinite(value)) values[name] = value;
        else invalid.push(name);
      }
    }
    return { values, invalid };
  }, [params, initialText]);

  const canSubmit =
    mode === "linear" || (params.ok && initial.invalid.length === 0);

  return (
    <InlineEditCard
      label="フィットの式を編集"
      editing={editing}
      onStartEditing={() => setEditing(true)}
      onCancel={() => setEditing(false)}
      formAction={formAction}
      pending={pending}
      submitLabel="保存してフィット"
      pendingLabel="フィット中..."
      submitDisabled={!canSubmit}
      error={state.error}
    >
      <div className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
        フィットの式
      </div>
      <input type="hidden" name="id" value={id} />
      <input type="hidden" name="mode" value={mode} />
      <input type="hidden" name="formula" value={formula} />
      <input
        type="hidden"
        name="initial"
        value={JSON.stringify(initial.values)}
      />

      <div className="flex flex-wrap gap-4 text-sm text-zinc-700 dark:text-zinc-300">
        <label className="flex items-center gap-1.5">
          <input
            type="radio"
            checked={mode === "linear"}
            onChange={() => setMode("linear")}
          />
          直線（y = ax + b）
        </label>
        <label className="flex items-center gap-1.5">
          <input
            type="radio"
            checked={mode === "curve"}
            onChange={() => setMode("curve")}
          />
          理論式を入力
        </label>
      </div>

      {mode === "curve" && (
        <>
          <label className="flex flex-col gap-1.5 text-sm text-zinc-700 dark:text-zinc-300">
            <span className="flex items-center gap-1.5">
              y =
              <InfoTooltip text="x 以外の文字はすべてフィットで求めるパラメータになります。^ はべき乗、pi と e は定数、sin・exp・sqrt・ln などの関数が使えます（誤差伝播ツールと同じ書き方）" />
            </span>
            <input
              type="text"
              value={formula}
              onChange={(e) => setFormula(e.target.value)}
              placeholder="例: A*exp(-x/tau) + C"
              spellCheck={false}
              className="rounded-md border border-zinc-300 bg-transparent px-2 py-1.5 font-mono text-sm dark:border-zinc-700"
            />
          </label>

          {!params.ok ? (
            formula.trim() !== "" && (
              <p className="text-sm text-red-600 dark:text-red-400">
                {params.error}
              </p>
            )
          ) : (
            <div className="flex flex-col gap-1.5 text-sm text-zinc-700 dark:text-zinc-300">
              <span className="flex items-center gap-1.5">
                初期値（任意）
                <InfoTooltip text="空欄のパラメータは 1 から探し始めます。sin の角振動数など、答えの近くから始めないと別の解に落ちる式では、おおよその値を入れてください" />
              </span>
              <div className="flex flex-wrap gap-3">
                {params.names.map((name) => (
                  <label key={name} className="flex items-center gap-1.5">
                    <span className="font-mono">{name}</span>
                    <input
                      type="text"
                      inputMode="decimal"
                      value={initialText[name] ?? ""}
                      onChange={(e) =>
                        setInitialText((prev) => ({
                          ...prev,
                          [name]: e.target.value,
                        }))
                      }
                      placeholder="1"
                      aria-invalid={initial.invalid.includes(name)}
                      className="w-24 rounded-md border border-zinc-300 bg-transparent px-2 py-1 text-sm aria-invalid:border-red-500 dark:border-zinc-700"
                    />
                  </label>
                ))}
              </div>
              {initial.invalid.length > 0 && (
                <p className="text-sm text-red-600 dark:text-red-400">
                  {initial.invalid.join("・")} の初期値は数値で入力してください
                </p>
              )}
            </div>
          )}
        </>
      )}
    </InlineEditCard>
  );
}

// The curve-fit setting saved on an experiment (KAN-29): which model the
// saved chart fits, stored as `config.fit` next to the axis labels.
//
// Absent means the default straight-line fit. Present means the worker's
// `curve_fit` with the stored formula -- the same grammar the error
// propagation tool speaks, parsed here with lib/formula.ts so the editor can
// point at a mistake before anything is saved.

import { FormulaError, parseFormula } from "@/lib/formula";
import type { ExperimentConfig } from "@/lib/experiment";

export type FitConfig = {
  // Right-hand side of `y = ...`.
  formula: string;
  // Starting values by parameter name; parameters left out start at the
  // worker's default (1.0).
  initial: Record<string, number>;
};

export type FitParameters =
  { ok: true; names: string[] } | { ok: false; error: string };

/**
 * The parameters a formula would fit: every variable except `x`, in the
 * order they first appear.
 *
 * Mirrors the worker's own checks (backend/worker/app/analysis/curve_fit.py),
 * so a formula the editor accepts is one the worker accepts -- the editor
 * can say what is wrong while the user is still typing instead of after a
 * round trip.
 */
export function fitParameters(formula: string): FitParameters {
  if (formula.trim() === "") {
    return { ok: false, error: "式を入力してください" };
  }
  let variables: string[];
  try {
    variables = parseFormula(formula).variables;
  } catch (e) {
    if (e instanceof FormulaError) {
      return { ok: false, error: `${e.message}（${e.position + 1}文字目）` };
    }
    throw e;
  }
  if (variables.includes("y")) {
    return {
      ok: false,
      error:
        "y は測定値なので式の中では使えません。右辺だけを書いてください（例: a*x + b）",
    };
  }
  const names = variables.filter((v) => v !== "x");
  if (names.length === 0) {
    return {
      ok: false,
      error:
        "フィットするパラメータがありません（x 以外の変数が式にありません）",
    };
  }
  return { ok: true, names };
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

/**
 * Reads `config.fit`. Config is free-form JSON, so anything not shaped like
 * a fit -- including a formula that no longer parses -- reads as "no fit",
 * which falls back to the straight line rather than breaking the page.
 * Initial values the formula does not use are dropped for the same reason:
 * the worker rejects them, and a stale one should not cost the whole fit.
 */
export function readFitConfig(config: ExperimentConfig): FitConfig | null {
  const fit = config.fit;
  if (typeof fit !== "object" || fit === null || Array.isArray(fit)) {
    return null;
  }
  const { formula, initial } = fit as Record<string, unknown>;
  if (typeof formula !== "string") return null;
  const params = fitParameters(formula);
  if (!params.ok) return null;

  const cleanInitial: Record<string, number> = {};
  if (typeof initial === "object" && initial !== null) {
    for (const name of params.names) {
      const value = (initial as Record<string, unknown>)[name];
      if (isFiniteNumber(value)) cleanInitial[name] = value;
    }
  }
  return { formula, initial: cleanInitial };
}

export type ParsedFitForm =
  { ok: true; fit: FitConfig | null } | { ok: false; error: string };

/**
 * Validates what the fit editor posts: `mode` ("linear" | "curve"),
 * `formula`, and `initial` as a JSON object of name -> number.
 *
 * A form is just an HTTP request, so none of this is trusted to be what the
 * editor built. `fit: null` means "back to the straight line".
 */
export function parseFitForm(
  mode: FormDataEntryValue | null,
  formula: FormDataEntryValue | null,
  initial: FormDataEntryValue | null,
): ParsedFitForm {
  if (mode === "linear") return { ok: true, fit: null };
  if (mode !== "curve") return { ok: false, error: "フィットの種類が不正です" };

  const formulaText = typeof formula === "string" ? formula.trim() : "";
  const params = fitParameters(formulaText);
  if (!params.ok) return params;

  let parsedInitial: unknown = {};
  if (typeof initial === "string" && initial !== "") {
    try {
      parsedInitial = JSON.parse(initial);
    } catch {
      return { ok: false, error: "初期値が正しく送信されていません" };
    }
  }
  if (
    typeof parsedInitial !== "object" ||
    parsedInitial === null ||
    Array.isArray(parsedInitial)
  ) {
    return { ok: false, error: "初期値が正しく送信されていません" };
  }

  const cleanInitial: Record<string, number> = {};
  for (const [name, value] of Object.entries(parsedInitial)) {
    if (!params.names.includes(name)) {
      return { ok: false, error: `${name} は式に含まれないパラメータです` };
    }
    if (!isFiniteNumber(value)) {
      return { ok: false, error: `${name} の初期値は数値で入力してください` };
    }
    cleanInitial[name] = value;
  }
  return { ok: true, fit: { formula: formulaText, initial: cleanInitial } };
}

/**
 * `patch` laid over `current`, key by key.
 *
 * PATCH /experiments/{id}/config replaces the whole config, so an editor
 * that sends only its own keys would erase everyone else's: saving the axis
 * labels would silently drop the saved fit. Every config write goes through
 * this instead. A key set to undefined is removed.
 */
export function mergeConfig(
  current: ExperimentConfig,
  patch: Record<string, unknown>,
): ExperimentConfig {
  const merged: ExperimentConfig = { ...current };
  for (const [key, value] of Object.entries(patch)) {
    if (value === undefined) delete merged[key];
    else merged[key] = value;
  }
  return merged;
}

"use server";

import { redirect } from "next/navigation";
import { callGoApi, GoApiError } from "@/lib/api";
import type { LinearRegressionResult } from "@/lib/experiment";
import { parseColumnsField } from "@/lib/experimentForm";

type Experiment = { id: string };

// Shared by every form action below that calls the Go API and then either
// redirects (success, or /login when there's no session) or surfaces the
// error back to the form. redirect() throws internally (its return type is
// `never`), so the only values this can actually resolve to are errors.
async function submitAndRedirect<T>(
  path: string,
  init: RequestInit,
  redirectTo: (data: T) => string,
): Promise<{ error: string }> {
  let data: T | null;
  try {
    data = await callGoApi<T>(path, init);
  } catch (e) {
    if (e instanceof GoApiError) {
      return { error: e.message };
    }
    return { error: e instanceof Error ? e.message : String(e) };
  }

  if (!data) {
    redirect("/login");
  }

  redirect(redirectTo(data));
}

export type CreateExperimentState = { error?: string };

export async function createExperiment(
  _prevState: CreateExperimentState,
  formData: FormData,
): Promise<CreateExperimentState> {
  const titleInput = String(formData.get("title") ?? "").trim();
  const title = titleInput === "" ? null : titleInput;

  const parsedColumns = parseColumnsField(formData.get("columns"));
  if (!parsedColumns.ok) return { error: parsedColumns.error };

  const config = {
    x_axis_label: String(formData.get("xAxisLabel") ?? "").trim(),
    y_axis_label: String(formData.get("yAxisLabel") ?? "").trim(),
  };

  return submitAndRedirect<Experiment>(
    "/api/v1/experiments",
    {
      method: "POST",
      body: JSON.stringify({
        title,
        raw_data: { columns: parsedColumns.columns },
        config,
      }),
    },
    (experiment) => `/experiments/${experiment.id}`,
  );
}

export type UpdateAxisLabelsState = { error?: string };

export async function updateAxisLabels(
  _prevState: UpdateAxisLabelsState,
  formData: FormData,
): Promise<UpdateAxisLabelsState> {
  const id = String(formData.get("id") ?? "");
  if (!id) return { error: "実験IDが不正です" };

  const config = {
    x_axis_label: String(formData.get("xAxisLabel") ?? "").trim(),
    y_axis_label: String(formData.get("yAxisLabel") ?? "").trim(),
  };

  return submitAndRedirect<Experiment>(
    `/api/v1/experiments/${id}/config`,
    { method: "PATCH", body: JSON.stringify({ config }) },
    () => `/experiments/${id}`,
  );
}

export type UpdateRawDataState = { error?: string };

export async function updateRawData(
  _prevState: UpdateRawDataState,
  formData: FormData,
): Promise<UpdateRawDataState> {
  const id = String(formData.get("id") ?? "");
  if (!id) return { error: "実験IDが不正です" };

  const parsedColumns = parseColumnsField(formData.get("columns"));
  if (!parsedColumns.ok) return { error: parsedColumns.error };

  return submitAndRedirect<Experiment>(
    `/api/v1/experiments/${id}/raw_data`,
    {
      method: "PATCH",
      body: JSON.stringify({ raw_data: { columns: parsedColumns.columns } }),
    },
    () => `/experiments/${id}`,
  );
}

export async function deleteExperiment(formData: FormData): Promise<void> {
  const id = String(formData.get("id") ?? "");
  if (!id) return;

  let result: { id: string } | null;
  try {
    result = await callGoApi<{ id: string }>(`/api/v1/experiments/${id}`, {
      method: "DELETE",
    });
  } catch (e) {
    if (e instanceof GoApiError && e.status === 404) {
      redirect("/experiments");
    }
    throw e;
  }

  if (!result) {
    redirect("/login");
  }

  redirect("/experiments");
}

// Runs a linear_regression analysis for an experiment. Best-effort, same as
// the rest of the analyze integration: too little data, a fit that fails
// (e.g. every point dropped by a log-scale fit's positive-value filter),
// or no session at all just resolve to null rather than throwing, so a
// missing regression never breaks the chart itself. Called both from the
// server (the page's initial linear-scale fetch) and directly from the
// client (ExperimentChart re-fetching when the user switches to a log axis).
export async function fetchRegression(
  id: string,
  params: { x_log?: boolean; y_log?: boolean },
): Promise<LinearRegressionResult | null> {
  try {
    const res = await callGoApi<{
      type: string;
      result: LinearRegressionResult;
    }>(`/api/v1/experiments/${id}/analyze`, {
      method: "POST",
      body: JSON.stringify({ type: "linear_regression", params }),
    });
    return res?.result ?? null;
  } catch {
    return null;
  }
}

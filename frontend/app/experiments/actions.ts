"use server";

import { redirect } from "next/navigation";
import { callGoApi, GoApiError } from "@/lib/api";

export type CreateExperimentState = { error?: string };

type Experiment = { id: string };

export async function createExperiment(
  _prevState: CreateExperimentState,
  formData: FormData,
): Promise<CreateExperimentState> {
  const titleInput = String(formData.get("title") ?? "").trim();
  const title = titleInput === "" ? null : titleInput;
  const columnsJson = String(formData.get("columns") ?? "");

  let columns: Record<string, number[]>;
  try {
    columns = JSON.parse(columnsJson);
  } catch {
    return { error: "データが正しく貼り付けられていません" };
  }
  if (!columns.x || !columns.y) {
    return { error: "x列とy列のデータが必要です" };
  }

  const config = {
    x_axis_label: String(formData.get("xAxisLabel") ?? "").trim(),
    y_axis_label: String(formData.get("yAxisLabel") ?? "").trim(),
  };

  let experiment: Experiment | null;
  try {
    experiment = await callGoApi<Experiment>("/api/v1/experiments", {
      method: "POST",
      body: JSON.stringify({ title, raw_data: { columns }, config }),
    });
  } catch (e) {
    if (e instanceof GoApiError) {
      return { error: e.message };
    }
    return { error: e instanceof Error ? e.message : String(e) };
  }

  if (!experiment) {
    redirect("/login");
  }

  redirect(`/experiments/${experiment.id}`);
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

"use server";

import { redirect } from "next/navigation";
import { callGoApi, GoApiError } from "@/lib/api";

export type CreateExperimentState = { error?: string };

type Experiment = { id: string };

export async function createExperiment(
  _prevState: CreateExperimentState,
  formData: FormData,
): Promise<CreateExperimentState> {
  const title = String(formData.get("title") ?? "").trim();
  const columnsJson = String(formData.get("columns") ?? "");

  if (!title) {
    return { error: "タイトルを入力してください" };
  }

  let columns: Record<string, number[]>;
  try {
    columns = JSON.parse(columnsJson);
  } catch {
    return { error: "データが正しく貼り付けられていません" };
  }
  if (!columns.x || !columns.y) {
    return { error: "x列とy列のデータが必要です" };
  }

  let experiment: Experiment | null;
  try {
    experiment = await callGoApi<Experiment>("/api/v1/experiments", {
      method: "POST",
      body: JSON.stringify({ title, raw_data: { columns } }),
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

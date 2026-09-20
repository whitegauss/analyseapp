"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { callGoApi, GoApiError } from "@/lib/api";

type Project = { id: string };

// `ok` marks a completed submission, so the inline forms can close
// themselves. It cannot be inferred from the absence of an error: the
// initial state has no error either.
export type ProjectFormState = { error?: string; ok?: boolean };

// Shared by create/update below: both call the Go API and then either stay
// on the dashboard with the list refreshed, or hand the error back to the
// inline form. Unlike the experiment actions' submitAndRedirect, these do
// not navigate -- the project list is edited in place, so the page it is on
// only needs re-rendering.
async function submitAndRefresh(
  path: string,
  init: RequestInit,
): Promise<ProjectFormState> {
  let data: Project | null;
  try {
    data = await callGoApi<Project>(path, init);
  } catch (e) {
    if (e instanceof GoApiError) {
      return { error: e.message };
    }
    return { error: e instanceof Error ? e.message : String(e) };
  }

  if (!data) {
    redirect("/login");
  }

  revalidatePath("/");
  return { ok: true };
}

export async function createProject(
  _prevState: ProjectFormState,
  formData: FormData,
): Promise<ProjectFormState> {
  const title = String(formData.get("title") ?? "").trim();
  if (title === "") return { error: "プロジェクト名を入力してください" };

  const description = String(formData.get("description") ?? "").trim();

  return submitAndRefresh("/api/v1/projects", {
    method: "POST",
    body: JSON.stringify({ title, description }),
  });
}

export async function updateProject(
  _prevState: ProjectFormState,
  formData: FormData,
): Promise<ProjectFormState> {
  const id = String(formData.get("id") ?? "");
  if (!id) return { error: "プロジェクトIDが不正です" };

  const title = String(formData.get("title") ?? "").trim();
  if (title === "") return { error: "プロジェクト名を入力してください" };

  const description = String(formData.get("description") ?? "").trim();

  return submitAndRefresh(`/api/v1/projects/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ title, description }),
  });
}

// Deleting a project takes its experiments with it, through the
// ON DELETE CASCADE on experiments.project_id (PDR.md section 5). The
// confirmation UI is what says so; by the time this runs the user has
// already been told how many experiments go with it.
export async function deleteProject(formData: FormData): Promise<void> {
  const id = String(formData.get("id") ?? "");
  if (!id) return;

  let result: { id: string } | null;
  try {
    result = await callGoApi<{ id: string }>(`/api/v1/projects/${id}`, {
      method: "DELETE",
    });
  } catch (e) {
    // Already gone (another tab, say) is the state the caller wanted, so
    // refresh rather than surfacing an error.
    if (e instanceof GoApiError && e.status === 404) {
      revalidatePath("/");
      return;
    }
    throw e;
  }

  if (!result) {
    redirect("/login");
  }

  revalidatePath("/");
}

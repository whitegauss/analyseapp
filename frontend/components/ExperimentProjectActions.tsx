"use client";

import { copyExperiment, moveExperiment } from "@/app/experiments/actions";
import type { ProjectSummary } from "@/lib/dashboard";
import ProjectPickerCard from "./ProjectPickerCard";

type Props = {
  id: string;
  currentProjectId: string;
  projects: ProjectSummary[];
};

// The two ways an experiment can change project, side by side, because the
// difference between them is the thing users get wrong: copy leaves a
// second, independent experiment behind; move re-parents this one and
// keeps its id.
export default function ExperimentProjectActions({
  id,
  currentProjectId,
  projects,
}: Props) {
  return (
    <>
      <ProjectPickerCard
        experimentId={id}
        currentProjectId={currentProjectId}
        projects={projects}
        action={copyExperiment}
        label="他のプロジェクトへコピー"
        description="複製です。コピー元はこのまま残り、コピーは別の実験として独立します（片方を編集しても、もう片方は変わりません）。"
        submitLabel="コピーする"
        pendingLabel="コピー中..."
        includeCurrentProject
      />
      <ProjectPickerCard
        experimentId={id}
        currentProjectId={currentProjectId}
        projects={projects}
        action={moveExperiment}
        label="他のプロジェクトへ移動"
        description="この実験の所属を付け替えます。複製はされず、実験IDも変わらないので、既存のリンクやCSVダウンロードURLはそのまま使えます。"
        submitLabel="移動する"
        pendingLabel="移動中..."
        includeCurrentProject={false}
      />
    </>
  );
}

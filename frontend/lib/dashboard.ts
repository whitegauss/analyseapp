// Shapes the dashboard's two API reads into the project -> experiments tree
// the page renders. Kept here, away from the page, because it is the only
// part with rules worth pinning in a test (frontend tests cover pure
// functions only -- see Readme).

export type ProjectSummary = {
  id: string;
  title: string;
  description: string;
};

export type ExperimentSummary = {
  id: string;
  project_id: string;
  title: string | null;
  created_at: string;
};

export type ProjectWithExperiments = ProjectSummary & {
  experiments: ExperimentSummary[];
};

/**
 * Buckets experiments under the project each one belongs to.
 *
 * The dashboard reads `GET /api/v1/projects` and `GET /api/v1/experiments`
 * once each and joins them here, rather than reading
 * `GET /api/v1/projects/{id}/experiments` per project: the page shows a
 * count for every project, so it needs all of them anyway, and one request
 * per project would grow with the list.
 *
 * Both inputs keep the order the API gave them (most recently created
 * first), and so does the output -- projects in their order, each one's
 * experiments in theirs.
 *
 * An experiment naming a project that isn't in `projects` is left out.
 * experiments.project_id is NOT NULL and references projects, and both
 * reads are scoped to the same user, so the only way to get one is a
 * project deleted between the two requests -- and that deletion cascades to
 * its experiments, so leaving them out is what the next load shows anyway.
 */
export function groupExperimentsByProject(
  projects: ProjectSummary[],
  experiments: ExperimentSummary[],
): ProjectWithExperiments[] {
  const byProjectId = new Map<string, ExperimentSummary[]>(
    projects.map((project) => [project.id, []]),
  );

  for (const experiment of experiments) {
    byProjectId.get(experiment.project_id)?.push(experiment);
  }

  return projects.map((project) => ({
    ...project,
    experiments: byProjectId.get(project.id) ?? [],
  }));
}

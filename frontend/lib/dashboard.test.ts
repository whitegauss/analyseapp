import { describe, expect, it } from "vitest";
import {
  attachProjectTitles,
  groupExperimentsByProject,
  type ExperimentSummary,
  type ProjectSummary,
} from "./dashboard";

function project(id: string, title = id): ProjectSummary {
  return { id, title, description: "" };
}

function experiment(
  id: string,
  projectId: string,
  createdAt = "2026-09-20T00:00:00Z",
): ExperimentSummary {
  return { id, project_id: projectId, title: id, created_at: createdAt };
}

describe("groupExperimentsByProject", () => {
  it("buckets each experiment under its project", () => {
    const grouped = groupExperimentsByProject(
      [project("p1"), project("p2")],
      [experiment("e1", "p1"), experiment("e2", "p2"), experiment("e3", "p1")],
    );

    expect(grouped.map((p) => [p.id, p.experiments.map((e) => e.id)])).toEqual([
      ["p1", ["e1", "e3"]],
      ["p2", ["e2"]],
    ]);
  });

  it("keeps a project that holds nothing, with an empty list", () => {
    // The count next to an empty project is what tells the user it is safe
    // to delete, so it has to survive the grouping.
    const grouped = groupExperimentsByProject([project("p1")], []);

    expect(grouped).toEqual([{ ...project("p1"), experiments: [] }]);
  });

  it("preserves the order both lists arrived in", () => {
    // The API returns most recently created first, and the dashboard shows
    // them that way -- the grouping must not re-sort either level.
    const grouped = groupExperimentsByProject(
      [project("newer"), project("older")],
      [
        experiment("e-late", "older", "2026-09-20T00:00:00Z"),
        experiment("e-early", "older", "2026-01-01T00:00:00Z"),
      ],
    );

    expect(grouped.map((p) => p.id)).toEqual(["newer", "older"]);
    expect(grouped[1].experiments.map((e) => e.id)).toEqual([
      "e-late",
      "e-early",
    ]);
  });

  it("leaves out an experiment whose project is not in the list", () => {
    // Only reachable when a project was deleted between the two requests;
    // that deletion cascades to its experiments, so dropping them here
    // matches what the next load returns.
    const grouped = groupExperimentsByProject(
      [project("p1")],
      [experiment("e1", "p1"), experiment("orphan", "deleted-project")],
    );

    expect(grouped).toEqual([
      { ...project("p1"), experiments: [experiment("e1", "p1")] },
    ]);
  });

  it("does not treat an inherited property name as a known project", () => {
    // The lookup is a Map, not a plain object, so a project_id of
    // "constructor" or "__proto__" cannot collide with Object.prototype.
    const grouped = groupExperimentsByProject(
      [project("p1")],
      [experiment("e1", "constructor"), experiment("e2", "__proto__")],
    );

    expect(grouped).toEqual([{ ...project("p1"), experiments: [] }]);
  });

  it("returns an empty list when there are no projects", () => {
    expect(groupExperimentsByProject([], [experiment("e1", "p1")])).toEqual([]);
  });
});

describe("attachProjectTitles", () => {
  it("labels each experiment with its project's title", () => {
    const labelled = attachProjectTitles(
      [project("p1", "落下運動"), project("p2", "振り子")],
      [experiment("e1", "p1"), experiment("e2", "p2")],
    );

    expect(labelled.map((e) => [e.id, e.projectTitle])).toEqual([
      ["e1", "落下運動"],
      ["e2", "振り子"],
    ]);
  });

  it("keeps an experiment whose project is missing, with a null title", () => {
    // The opposite of groupExperimentsByProject: here the experiment is the
    // row, so it stays visible even when the label cannot be resolved.
    const labelled = attachProjectTitles(
      [],
      [experiment("e1", "deleted-project")],
    );

    expect(labelled).toEqual([
      { ...experiment("e1", "deleted-project"), projectTitle: null },
    ]);
  });

  it("does not resolve a title from an inherited property name", () => {
    const labelled = attachProjectTitles(
      [project("p1")],
      [experiment("e1", "constructor")],
    );

    expect(labelled[0].projectTitle).toBeNull();
  });

  it("keeps the experiments in the order they arrived in", () => {
    const labelled = attachProjectTitles(
      [project("p1")],
      [experiment("e2", "p1"), experiment("e1", "p1")],
    );

    expect(labelled.map((e) => e.id)).toEqual(["e2", "e1"]);
  });
});

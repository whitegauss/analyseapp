//go:build integration

// The two queries that move data between projects (KAN-26 / KAN-87). Kept
// apart from repository_integration_test.go because these are the newest
// and least-exercised SQL in the package, and because both guard *two*
// ownerships in one statement -- the source's and the destination's --
// which is a different thing to get wrong than a plain "and user_id = $2".
package experiments_test

import (
	"context"
	"errors"
	"testing"

	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/projects"
)

func TestCopy(t *testing.T) {
	t.Run("duplicates the data under a new id in the destination", func(t *testing.T) {
		f := newFixture(t)
		source := f.newExperiment(t, "元の測定")
		destination, err := projects.NewRepository(f.pool).
			Create(context.Background(), f.userID, "コピー先", "")
		if err != nil {
			t.Fatalf("create destination: %v", err)
		}

		copied, err := f.repo.Copy(context.Background(), source.ID, f.userID, destination.ID)
		if err != nil {
			t.Fatalf("Copy: %v", err)
		}

		if copied.ID == source.ID {
			t.Error("the copy reuses the source's id, want a new one")
		}
		if copied.ProjectID != destination.ID {
			t.Errorf("project = %v, want the destination %v", copied.ProjectID, destination.ID)
		}
		if copied.Title == nil || *copied.Title != "元の測定" {
			t.Errorf("title = %v, want the source's", copied.Title)
		}
		if copied.Config["x_axis_label"] != "t" {
			t.Errorf("config = %v, want the source's", copied.Config)
		}

		// The source has to survive untouched -- that is the whole
		// difference from a move.
		if _, err := f.repo.GetByID(context.Background(), source.ID, f.userID); err != nil {
			t.Errorf("the source is gone after a copy: %v", err)
		}
	})

	t.Run("another user's source copies nothing", func(t *testing.T) {
		f := newFixture(t)
		theirUserID, theirProject := stranger(t, f.pool)
		theirExperiment, err := experiments.NewRepository(f.pool).Create(context.Background(),
			theirUserID, theirProject.ID, nil, map[string]any{"columns": map[string]any{}}, nil)
		if err != nil {
			t.Fatalf("create stranger's experiment: %v", err)
		}

		_, err = f.repo.Copy(context.Background(), theirExperiment.ID, f.userID, f.project.ID)
		if !errors.Is(err, experiments.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}

		list, err := f.repo.ListByProject(context.Background(), f.project.ID, f.userID)
		if err != nil {
			t.Fatalf("ListByProject: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("%d rows appeared from another user's experiment, want 0", len(list))
		}
	})

	t.Run("another user's destination copies nothing", func(t *testing.T) {
		f := newFixture(t)
		source := f.newExperiment(t, "測定")
		_, theirProject := stranger(t, f.pool)

		_, err := f.repo.Copy(context.Background(), source.ID, f.userID, theirProject.ID)
		if !errors.Is(err, experiments.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}

		var count int
		if err := f.pool.QueryRow(context.Background(),
			`select count(*) from experiments where project_id = $1`, theirProject.ID).Scan(&count); err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 0 {
			t.Errorf("%d rows written into the other user's project, want 0", count)
		}
	})
}

func TestUpdateProject(t *testing.T) {
	t.Run("re-parents the row and keeps its id", func(t *testing.T) {
		f := newFixture(t)
		e := f.newExperiment(t, "測定")
		destination, err := projects.NewRepository(f.pool).
			Create(context.Background(), f.userID, "移動先", "")
		if err != nil {
			t.Fatalf("create destination: %v", err)
		}

		moved, err := f.repo.UpdateProject(context.Background(), e.ID, f.userID, destination.ID)
		if err != nil {
			t.Fatalf("UpdateProject: %v", err)
		}

		// Keeping the id is the entire reason this exists rather than
		// copy-then-delete (KAN-87).
		if moved.ID != e.ID {
			t.Errorf("id = %v, want it unchanged at %v", moved.ID, e.ID)
		}
		if moved.ProjectID != destination.ID {
			t.Errorf("project = %v, want %v", moved.ProjectID, destination.ID)
		}
		if !moved.CreatedAt.Equal(e.CreatedAt) {
			t.Errorf("created_at = %v, want it unchanged at %v", moved.CreatedAt, e.CreatedAt)
		}

		// And exactly one row exists afterwards, in the new project.
		old, err := f.repo.ListByProject(context.Background(), f.project.ID, f.userID)
		if err != nil {
			t.Fatalf("ListByProject: %v", err)
		}
		if len(old) != 0 {
			t.Errorf("%d rows left in the old project, want 0", len(old))
		}
	})

	t.Run("moving to the project it is already in changes nothing", func(t *testing.T) {
		f := newFixture(t)
		e := f.newExperiment(t, "測定")

		moved, err := f.repo.UpdateProject(context.Background(), e.ID, f.userID, f.project.ID)
		if err != nil {
			t.Fatalf("UpdateProject: %v", err)
		}
		if moved.ID != e.ID || moved.ProjectID != f.project.ID {
			t.Errorf("got %v in %v, want %v in %v", moved.ID, moved.ProjectID, e.ID, f.project.ID)
		}
	})

	t.Run("another user cannot move it", func(t *testing.T) {
		f := newFixture(t)
		e := f.newExperiment(t, "測定")
		theirUserID, theirProject := stranger(t, f.pool)

		_, err := experiments.NewRepository(f.pool).
			UpdateProject(context.Background(), e.ID, theirUserID, theirProject.ID)
		if !errors.Is(err, experiments.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}

		got, err := f.repo.GetByID(context.Background(), e.ID, f.userID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.ProjectID != f.project.ID {
			t.Errorf("project = %v, want it still in %v", got.ProjectID, f.project.ID)
		}
	})

	t.Run("cannot move into another user's project", func(t *testing.T) {
		f := newFixture(t)
		e := f.newExperiment(t, "測定")
		_, theirProject := stranger(t, f.pool)

		_, err := f.repo.UpdateProject(context.Background(), e.ID, f.userID, theirProject.ID)
		if !errors.Is(err, experiments.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}

		got, err := f.repo.GetByID(context.Background(), e.ID, f.userID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.ProjectID != f.project.ID {
			t.Errorf("project = %v, want it still in %v", got.ProjectID, f.project.ID)
		}
	})
}

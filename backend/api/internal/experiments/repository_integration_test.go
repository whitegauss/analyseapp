//go:build integration

// Tests for the SQL itself, against a real Postgres (KAN-31). The handler
// tests next door use a fake store, which by construction cannot notice a
// query that stopped filtering by owner, dropped a column, or violated a
// constraint -- a fake returns whatever the test told it to.
//
// Run with:
//
//	TEST_DATABASE_URL=... go test -tags=integration ./internal/experiments/
package experiments_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/pgtest"
	"analyseapp/api/internal/projects"
)

// fixture is one user with one project, the usual starting point: an
// experiment cannot exist without both.
type fixture struct {
	pool    *pgxpool.Pool
	repo    *experiments.Repository
	userID  uuid.UUID
	project projects.Project
}

func newFixture(t *testing.T) fixture {
	t.Helper()

	pool := pgtest.Pool(t)
	userID := pgtest.NewUser(t, pool)

	project, err := projects.NewRepository(pool).
		Create(context.Background(), userID, "落下運動の実験", "")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	return fixture{
		pool:    pool,
		repo:    experiments.NewRepository(pool),
		userID:  userID,
		project: project,
	}
}

// newExperiment stores one experiment in f's project and returns it.
func (f fixture) newExperiment(t *testing.T, title string) experiments.Experiment {
	t.Helper()

	e, err := f.repo.Create(context.Background(), f.userID, f.project.ID, &title,
		map[string]any{"columns": map[string]any{"x": []any{1, 2}, "y": []any{3, 4}}},
		map[string]any{"x_axis_label": "t"},
	)
	if err != nil {
		t.Fatalf("create experiment: %v", err)
	}
	return e
}

// stranger is another user with their own project -- the "someone else"
// every authorization check below is measured against.
func stranger(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, projects.Project) {
	t.Helper()

	userID := pgtest.NewUser(t, pool)
	project, err := projects.NewRepository(pool).
		Create(context.Background(), userID, "他人のプロジェクト", "")
	if err != nil {
		t.Fatalf("create stranger's project: %v", err)
	}
	return userID, project
}

func TestEnsureProfile(t *testing.T) {
	pool := pgtest.Pool(t)
	repo := experiments.NewRepository(pool)
	ctx := context.Background()

	// No profiles row yet, only the Supabase-side user: the state a user's
	// very first write arrives in.
	userID := pgtest.NewAuthUser(t, pool)

	if err := repo.EnsureProfile(ctx, userID); err != nil {
		t.Fatalf("EnsureProfile: %v", err)
	}
	// Called again on the next write, and every write after that, so the
	// upsert has to stay quiet rather than violating the primary key.
	if err := repo.EnsureProfile(ctx, userID); err != nil {
		t.Fatalf("EnsureProfile (second call): %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`select count(*) from profiles where id = $1`, userID).Scan(&count); err != nil {
		t.Fatalf("count profiles: %v", err)
	}
	if count != 1 {
		t.Errorf("profiles rows = %d, want exactly 1", count)
	}
}

func TestCreate(t *testing.T) {
	t.Run("stores the row and returns it", func(t *testing.T) {
		f := newFixture(t)
		title := "1回目の測定"
		rawData := map[string]any{"columns": map[string]any{"x": []any{1.5}}}

		e, err := f.repo.Create(context.Background(), f.userID, f.project.ID,
			&title, rawData, map[string]any{"x_axis_label": "t"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		if e.ID == uuid.Nil {
			t.Error("id was not assigned")
		}
		if e.UserID != f.userID || e.ProjectID != f.project.ID {
			t.Errorf("owner/project = %v/%v, want %v/%v", e.UserID, e.ProjectID, f.userID, f.project.ID)
		}
		if e.Title == nil || *e.Title != title {
			t.Errorf("title = %v, want %q", e.Title, title)
		}
		if e.CreatedAt.IsZero() || e.UpdatedAt.IsZero() {
			t.Error("timestamps were not defaulted")
		}

		// Read it back through a separate query: Create returning the right
		// struct says nothing about what landed in the table.
		got, err := f.repo.GetByID(context.Background(), e.ID, f.userID)
		if err != nil {
			t.Fatalf("GetByID after Create: %v", err)
		}
		if got.ID != e.ID {
			t.Errorf("stored id = %v, want %v", got.ID, e.ID)
		}
	})

	t.Run("a nil title is stored as null", func(t *testing.T) {
		// experiments.title was made nullable by migration 00005, and the
		// UI relies on it for "not named yet".
		f := newFixture(t)

		e, err := f.repo.Create(context.Background(), f.userID, f.project.ID,
			nil, map[string]any{"columns": map[string]any{}}, nil)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if e.Title != nil {
			t.Errorf("title = %q, want null", *e.Title)
		}
	})

	t.Run("an empty title is stored as an empty string, not null", func(t *testing.T) {
		// Pinning the division of labour rather than endorsing it: the
		// handler is what turns "" into nil (parseCreateExperimentRequest),
		// and the repository stores exactly what it is handed. If the
		// normalization ever moves down here, this test should change with
		// it deliberately.
		f := newFixture(t)
		empty := ""

		e, err := f.repo.Create(context.Background(), f.userID, f.project.ID,
			&empty, map[string]any{"columns": map[string]any{}}, nil)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if e.Title == nil {
			t.Fatal("title = null, want an empty string")
		}
		if *e.Title != "" {
			t.Errorf("title = %q, want an empty string", *e.Title)
		}
	})

	t.Run("a nil config is stored as an empty object", func(t *testing.T) {
		f := newFixture(t)

		e, err := f.repo.Create(context.Background(), f.userID, f.project.ID,
			nil, map[string]any{"columns": map[string]any{}}, nil)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if e.Config == nil {
			t.Error("config = nil, want an empty object so callers can serialize it")
		}
		if len(e.Config) != 0 {
			t.Errorf("config = %v, want empty", e.Config)
		}
	})

	// The guard that makes the nested create path safe: without it, posting
	// a guessed project id would file an experiment into someone else's
	// project (KAN-25).
	t.Run("another user's project writes nothing", func(t *testing.T) {
		f := newFixture(t)
		_, theirProject := stranger(t, f.pool)

		_, err := f.repo.Create(context.Background(), f.userID, theirProject.ID,
			nil, map[string]any{"columns": map[string]any{}}, nil)
		if !errors.Is(err, experiments.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}

		var count int
		if err := f.pool.QueryRow(context.Background(),
			`select count(*) from experiments where project_id = $1`,
			theirProject.ID).Scan(&count); err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 0 {
			t.Errorf("%d rows written into the other user's project, want 0", count)
		}
	})

	t.Run("a project that does not exist writes nothing", func(t *testing.T) {
		f := newFixture(t)

		_, err := f.repo.Create(context.Background(), f.userID, uuid.New(),
			nil, map[string]any{"columns": map[string]any{}}, nil)
		if !errors.Is(err, experiments.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestGetByID(t *testing.T) {
	t.Run("another user's experiment is not found", func(t *testing.T) {
		f := newFixture(t)
		e := f.newExperiment(t, "秘密の測定")
		theirUserID, _ := stranger(t, f.pool)

		_, err := f.repo.GetByID(context.Background(), e.ID, theirUserID)
		if !errors.Is(err, experiments.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("an unknown id is not found", func(t *testing.T) {
		f := newFixture(t)

		_, err := f.repo.GetByID(context.Background(), uuid.New(), f.userID)
		if !errors.Is(err, experiments.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("round-trips raw_data and config", func(t *testing.T) {
		// jsonb goes through pgx's own encoding in both directions, so this
		// is the only place the shapes the app stores are checked at all.
		f := newFixture(t)
		title := "測定"
		rawData := map[string]any{
			"columns": map[string]any{"x": []any{1.0, 2.5}, "y_error": []any{0.1, 0.2}},
		}
		config := map[string]any{"x_axis_label": "時刻 $t$ (s)", "legend": map[string]any{"size": 12.0}}

		created, err := f.repo.Create(context.Background(), f.userID, f.project.ID, &title, rawData, config)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, err := f.repo.GetByID(context.Background(), created.ID, f.userID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}

		columns, ok := got.RawData["columns"].(map[string]any)
		if !ok {
			t.Fatalf("raw_data.columns = %#v, want an object", got.RawData["columns"])
		}
		if x, ok := columns["x"].([]any); !ok || len(x) != 2 || x[1] != 2.5 {
			t.Errorf("raw_data.columns.x = %#v, want [1 2.5]", columns["x"])
		}
		if got.Config["x_axis_label"] != "時刻 $t$ (s)" {
			t.Errorf("config.x_axis_label = %#v, want the multibyte label back unchanged", got.Config["x_axis_label"])
		}
	})
}

func TestListByUser(t *testing.T) {
	t.Run("newest first, and only this user's", func(t *testing.T) {
		f := newFixture(t)
		first := f.newExperiment(t, "1回目")
		second := f.newExperiment(t, "2回目")

		theirUserID, theirProject := stranger(t, f.pool)
		if _, err := experiments.NewRepository(f.pool).Create(context.Background(),
			theirUserID, theirProject.ID, nil, map[string]any{"columns": map[string]any{}}, nil); err != nil {
			t.Fatalf("create stranger's experiment: %v", err)
		}

		list, err := f.repo.ListByUser(context.Background(), f.userID)
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}

		if len(list) != 2 {
			t.Fatalf("got %d experiments, want exactly this user's 2", len(list))
		}
		if list[0].ID != second.ID || list[1].ID != first.ID {
			t.Errorf("order = %v, want newest first (%v, %v)",
				[]uuid.UUID{list[0].ID, list[1].ID}, second.ID, first.ID)
		}
	})

	t.Run("a user with nothing gets an empty slice, not nil", func(t *testing.T) {
		// Handlers serialize this straight to JSON; nil would become null
		// where the clients expect [].
		pool := pgtest.Pool(t)
		userID := pgtest.NewUser(t, pool)

		list, err := experiments.NewRepository(pool).ListByUser(context.Background(), userID)
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}
		if list == nil {
			t.Fatal("list is nil, want an empty slice")
		}
		if len(list) != 0 {
			t.Errorf("len = %d, want 0", len(list))
		}
	})
}

func TestListByProject(t *testing.T) {
	t.Run("only the named project's experiments", func(t *testing.T) {
		f := newFixture(t)
		inProject := f.newExperiment(t, "このプロジェクトの実験")

		other, err := projects.NewRepository(f.pool).
			Create(context.Background(), f.userID, "別のプロジェクト", "")
		if err != nil {
			t.Fatalf("create second project: %v", err)
		}
		if _, err := f.repo.Create(context.Background(), f.userID, other.ID, nil,
			map[string]any{"columns": map[string]any{}}, nil); err != nil {
			t.Fatalf("create experiment in second project: %v", err)
		}

		list, err := f.repo.ListByProject(context.Background(), f.project.ID, f.userID)
		if err != nil {
			t.Fatalf("ListByProject: %v", err)
		}
		if len(list) != 1 || list[0].ID != inProject.ID {
			t.Errorf("got %d experiments (%v), want only %v", len(list), list, inProject.ID)
		}
	})

	// ListByProject filters on user_id as well as project_id, so even a
	// correctly guessed project id yields nothing. The handler's separate
	// 404 is about the message, not about this.
	t.Run("another user's project yields nothing", func(t *testing.T) {
		f := newFixture(t)
		theirUserID, theirProject := stranger(t, f.pool)
		if _, err := experiments.NewRepository(f.pool).Create(context.Background(),
			theirUserID, theirProject.ID, nil, map[string]any{"columns": map[string]any{}}, nil); err != nil {
			t.Fatalf("create stranger's experiment: %v", err)
		}

		list, err := f.repo.ListByProject(context.Background(), theirProject.ID, f.userID)
		if err != nil {
			t.Fatalf("ListByProject: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("got %d of another user's experiments, want 0", len(list))
		}
	})
}

func TestUpdateConfigAndRawData(t *testing.T) {
	t.Run("another user cannot update", func(t *testing.T) {
		f := newFixture(t)
		e := f.newExperiment(t, "測定")
		theirUserID, _ := stranger(t, f.pool)

		if _, err := f.repo.UpdateConfig(context.Background(), e.ID, theirUserID,
			map[string]any{"x_axis_label": "乗っ取り"}); !errors.Is(err, experiments.ErrNotFound) {
			t.Errorf("UpdateConfig err = %v, want ErrNotFound", err)
		}
		if _, err := f.repo.UpdateRawData(context.Background(), e.ID, theirUserID,
			map[string]any{"columns": map[string]any{}}); !errors.Is(err, experiments.ErrNotFound) {
			t.Errorf("UpdateRawData err = %v, want ErrNotFound", err)
		}

		got, err := f.repo.GetByID(context.Background(), e.ID, f.userID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.Config["x_axis_label"] != "t" {
			t.Errorf("config = %v, want it untouched by the other user", got.Config)
		}
	})

	t.Run("the owner's update replaces the field and moves updated_at", func(t *testing.T) {
		f := newFixture(t)
		e := f.newExperiment(t, "測定")

		updated, err := f.repo.UpdateConfig(context.Background(), e.ID, f.userID,
			map[string]any{"y_axis_label": "v"})
		if err != nil {
			t.Fatalf("UpdateConfig: %v", err)
		}

		// Replaced wholesale, not merged -- the handler documents it as a
		// full replacement and the UI sends the complete object.
		if _, stillThere := updated.Config["x_axis_label"]; stillThere {
			t.Errorf("config = %v, want the old keys gone", updated.Config)
		}
		if updated.Config["y_axis_label"] != "v" {
			t.Errorf("config.y_axis_label = %v, want v", updated.Config["y_axis_label"])
		}
		if !updated.UpdatedAt.After(e.UpdatedAt) {
			t.Errorf("updated_at = %v, want later than %v", updated.UpdatedAt, e.UpdatedAt)
		}
		if !updated.CreatedAt.Equal(e.CreatedAt) {
			t.Errorf("created_at = %v, want it unchanged at %v", updated.CreatedAt, e.CreatedAt)
		}
	})
}

func TestDelete(t *testing.T) {
	t.Run("another user cannot delete", func(t *testing.T) {
		f := newFixture(t)
		e := f.newExperiment(t, "消されたくない測定")
		theirUserID, _ := stranger(t, f.pool)

		if err := f.repo.Delete(context.Background(), e.ID, theirUserID); !errors.Is(err, experiments.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
		if _, err := f.repo.GetByID(context.Background(), e.ID, f.userID); err != nil {
			t.Errorf("the experiment is gone after another user's delete: %v", err)
		}
	})

	t.Run("an unknown id is not found", func(t *testing.T) {
		f := newFixture(t)

		if err := f.repo.Delete(context.Background(), uuid.New(), f.userID); !errors.Is(err, experiments.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	// The cascade the Delete doc comment promises instead of cleaning up in
	// application code (00004_create_analysis_results.sql).
	t.Run("deleting an experiment removes its analysis_results", func(t *testing.T) {
		f := newFixture(t)
		e := f.newExperiment(t, "測定")

		if _, err := f.pool.Exec(context.Background(),
			`insert into analysis_results (experiment_id, analysis_type) values ($1, 'linear_regression')`,
			e.ID); err != nil {
			t.Fatalf("insert analysis_result: %v", err)
		}

		if err := f.repo.Delete(context.Background(), e.ID, f.userID); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		var count int
		if err := f.pool.QueryRow(context.Background(),
			`select count(*) from analysis_results where experiment_id = $1`, e.ID).Scan(&count); err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 0 {
			t.Errorf("%d analysis_results left behind, want 0", count)
		}
	})
}

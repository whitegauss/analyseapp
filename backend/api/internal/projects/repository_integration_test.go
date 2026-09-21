//go:build integration

// Tests for the projects SQL against a real Postgres (KAN-31). See the
// note in internal/experiments/repository_integration_test.go for why the
// fake-store handler tests are not enough on their own.
//
// Run with:
//
//	TEST_DATABASE_URL=... go test -tags=integration ./internal/projects/
package projects_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/pgtest"
	"analyseapp/api/internal/projects"
)

func newRepo(t *testing.T) (*projects.Repository, *pgxpool.Pool, uuid.UUID) {
	t.Helper()

	pool := pgtest.Pool(t)
	return projects.NewRepository(pool), pool, pgtest.NewUser(t, pool)
}

func mustCreate(t *testing.T, repo *projects.Repository, userID uuid.UUID, title string) projects.Project {
	t.Helper()

	p, err := repo.Create(context.Background(), userID, title, "")
	if err != nil {
		t.Fatalf("create project %q: %v", title, err)
	}
	return p
}

func TestEnsureProfile(t *testing.T) {
	// Duplicated from experiments.Repository on purpose (see the doc
	// comment there), which means it needs its own coverage: creating a
	// project can be a user's very first write, before any experiment
	// exists to have made the profiles row.
	pool := pgtest.Pool(t)
	repo := projects.NewRepository(pool)
	ctx := context.Background()
	userID := pgtest.NewAuthUser(t, pool)

	if err := repo.EnsureProfile(ctx, userID); err != nil {
		t.Fatalf("EnsureProfile: %v", err)
	}
	if _, err := repo.Create(ctx, userID, "最初のプロジェクト", ""); err != nil {
		t.Fatalf("Create after EnsureProfile: %v", err)
	}
}

func TestCreateAndGetByID(t *testing.T) {
	t.Run("stores the row and returns it", func(t *testing.T) {
		repo, _, userID := newRepo(t)

		p, err := repo.Create(context.Background(), userID, "落下運動の実験", "力学レポート用")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if p.ID == uuid.Nil {
			t.Error("id was not assigned")
		}
		if p.Title != "落下運動の実験" || p.Description != "力学レポート用" {
			t.Errorf("got %q/%q, want the values passed in", p.Title, p.Description)
		}

		got, err := repo.GetByID(context.Background(), p.ID, userID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.ID != p.ID {
			t.Errorf("stored id = %v, want %v", got.ID, p.ID)
		}
	})

	t.Run("another user's project is not found", func(t *testing.T) {
		repo, pool, userID := newRepo(t)
		p := mustCreate(t, repo, userID, "秘密のプロジェクト")
		theirUserID := pgtest.NewUser(t, pool)

		_, err := repo.GetByID(context.Background(), p.ID, theirUserID)
		if !errors.Is(err, projects.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("an unknown id is not found", func(t *testing.T) {
		repo, _, userID := newRepo(t)

		_, err := repo.GetByID(context.Background(), uuid.New(), userID)
		if !errors.Is(err, projects.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})
}

func TestListByUser(t *testing.T) {
	t.Run("newest first, and only this user's", func(t *testing.T) {
		repo, pool, userID := newRepo(t)
		first := mustCreate(t, repo, userID, "1つ目")
		second := mustCreate(t, repo, userID, "2つ目")

		theirUserID := pgtest.NewUser(t, pool)
		mustCreate(t, repo, theirUserID, "他人のプロジェクト")

		list, err := repo.ListByUser(context.Background(), userID)
		if err != nil {
			t.Fatalf("ListByUser: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("got %d projects, want exactly this user's 2", len(list))
		}
		if list[0].ID != second.ID || list[1].ID != first.ID {
			t.Errorf("order = %v, want newest first (%v, %v)",
				[]uuid.UUID{list[0].ID, list[1].ID}, second.ID, first.ID)
		}
	})

	t.Run("a user with nothing gets an empty slice, not nil", func(t *testing.T) {
		repo, _, userID := newRepo(t)

		list, err := repo.ListByUser(context.Background(), userID)
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

func TestUpdate(t *testing.T) {
	t.Run("another user cannot rename it", func(t *testing.T) {
		repo, pool, userID := newRepo(t)
		p := mustCreate(t, repo, userID, "元の名前")
		theirUserID := pgtest.NewUser(t, pool)

		_, err := repo.Update(context.Background(), p.ID, theirUserID, "乗っ取り", "")
		if !errors.Is(err, projects.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}

		got, err := repo.GetByID(context.Background(), p.ID, userID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.Title != "元の名前" {
			t.Errorf("title = %q, want it untouched", got.Title)
		}
	})

	t.Run("the owner's rename lands and moves updated_at", func(t *testing.T) {
		repo, _, userID := newRepo(t)
		p := mustCreate(t, repo, userID, "元の名前")

		updated, err := repo.Update(context.Background(), p.ID, userID, "新しい名前", "説明も")
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if updated.Title != "新しい名前" || updated.Description != "説明も" {
			t.Errorf("got %q/%q, want the new values", updated.Title, updated.Description)
		}
		if !updated.UpdatedAt.After(p.UpdatedAt) {
			t.Errorf("updated_at = %v, want later than %v", updated.UpdatedAt, p.UpdatedAt)
		}
	})
}

func TestDelete(t *testing.T) {
	t.Run("another user cannot delete it", func(t *testing.T) {
		repo, pool, userID := newRepo(t)
		p := mustCreate(t, repo, userID, "消されたくない")
		theirUserID := pgtest.NewUser(t, pool)

		if err := repo.Delete(context.Background(), p.ID, theirUserID); !errors.Is(err, projects.ErrNotFound) {
			t.Fatalf("err = %v, want ErrNotFound", err)
		}
		if _, err := repo.GetByID(context.Background(), p.ID, userID); err != nil {
			t.Errorf("the project is gone after another user's delete: %v", err)
		}
	})

	// The cascade the delete confirmation UI warns about (PDR.md section
	// 5). If this stopped working the FK would instead refuse the delete,
	// so the failure mode is loud -- but the warning text would also be a
	// lie, which is worth pinning either way.
	t.Run("deleting a project takes its experiments with it", func(t *testing.T) {
		repo, pool, userID := newRepo(t)
		p := mustCreate(t, repo, userID, "消すプロジェクト")

		e, err := experiments.NewRepository(pool).Create(context.Background(),
			userID, p.ID, nil, map[string]any{"columns": map[string]any{}}, nil)
		if err != nil {
			t.Fatalf("create experiment: %v", err)
		}

		if err := repo.Delete(context.Background(), p.ID, userID); err != nil {
			t.Fatalf("Delete: %v", err)
		}

		_, err = experiments.NewRepository(pool).GetByID(context.Background(), e.ID, userID)
		if !errors.Is(err, experiments.ErrNotFound) {
			t.Errorf("the experiment survived its project's deletion (err = %v)", err)
		}
	})
}

func TestEnsureDefault(t *testing.T) {
	t.Run("creates the default project, then returns the same one", func(t *testing.T) {
		repo, _, userID := newRepo(t)

		first, err := repo.EnsureDefault(context.Background(), userID)
		if err != nil {
			t.Fatalf("EnsureDefault: %v", err)
		}
		if first.Title != projects.DefaultTitle {
			t.Errorf("title = %q, want %q", first.Title, projects.DefaultTitle)
		}

		second, err := repo.EnsureDefault(context.Background(), userID)
		if err != nil {
			t.Fatalf("EnsureDefault (second call): %v", err)
		}
		if second.ID != first.ID {
			t.Errorf("second call returned %v, want the same project %v", second.ID, first.ID)
		}
	})

	// The reason the insert carries ON CONFLICT DO NOTHING: two requests
	// arriving together must not leave the user with two default projects,
	// because the migration and the API would then disagree about which
	// one "未分類" means. The partial unique index is what makes the loser
	// lose; this checks both halves actually work together.
	t.Run("concurrent first requests still produce one project", func(t *testing.T) {
		repo, pool, userID := newRepo(t)

		const callers = 8
		ids := make([]uuid.UUID, callers)
		errs := make([]error, callers)
		var wg sync.WaitGroup
		for i := range callers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p, err := repo.EnsureDefault(context.Background(), userID)
				ids[i], errs[i] = p.ID, err
			}()
		}
		wg.Wait()

		for i, err := range errs {
			if err != nil {
				t.Fatalf("caller %d: %v", i, err)
			}
		}
		for i, id := range ids {
			if id != ids[0] {
				t.Errorf("caller %d got project %v, want the same one as caller 0 (%v)", i, id, ids[0])
			}
		}

		var count int
		if err := pool.QueryRow(context.Background(),
			`select count(*) from projects where user_id = $1 and title = $2`,
			userID, projects.DefaultTitle).Scan(&count); err != nil {
			t.Fatalf("count: %v", err)
		}
		if count != 1 {
			t.Errorf("%d default projects exist, want exactly 1", count)
		}
	})

	// projects_user_default_idx (migration 00007) is a partial unique index
	// on title = '未分類'. Ordinary projects are not covered by it, so a
	// user is still free to name two of their own projects the same thing.
	t.Run("the uniqueness only covers the default title", func(t *testing.T) {
		repo, _, userID := newRepo(t)

		mustCreate(t, repo, userID, "同じ名前")
		if _, err := repo.Create(context.Background(), userID, "同じ名前", ""); err != nil {
			t.Errorf("a second project with the same ordinary name was refused: %v", err)
		}

		if _, err := repo.EnsureDefault(context.Background(), userID); err != nil {
			t.Fatalf("EnsureDefault: %v", err)
		}
		if _, err := repo.Create(context.Background(), userID, projects.DefaultTitle, ""); err == nil {
			t.Error("a second 未分類 was accepted, want the partial unique index to refuse it")
		}
	})

	t.Run("each user gets their own default project", func(t *testing.T) {
		repo, pool, userID := newRepo(t)
		theirUserID := pgtest.NewUser(t, pool)

		mine, err := repo.EnsureDefault(context.Background(), userID)
		if err != nil {
			t.Fatalf("EnsureDefault: %v", err)
		}
		theirs, err := repo.EnsureDefault(context.Background(), theirUserID)
		if err != nil {
			t.Fatalf("EnsureDefault (other user): %v", err)
		}

		if mine.ID == theirs.ID {
			t.Error("both users were handed the same default project")
		}
		if theirs.UserID != theirUserID {
			t.Errorf("owner = %v, want %v", theirs.UserID, theirUserID)
		}
	})
}

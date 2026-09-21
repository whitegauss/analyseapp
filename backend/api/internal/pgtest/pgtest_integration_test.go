//go:build integration

// Checks the harness itself. Without this, a broken Pool would show up as
// every repository test failing at once, and the first PR to land the
// harness would have nothing proving the CI wiring actually works.
package pgtest_test

import (
	"context"
	"testing"

	"analyseapp/api/internal/pgtest"
)

// TestPoolAppliesMigrations asserts the schema arrived, not just that the
// connection did. goose reporting success is not the same as the tables
// existing under the names the repositories query.
func TestPoolAppliesMigrations(t *testing.T) {
	pool := pgtest.Pool(t)

	for _, table := range []string{"profiles", "experiments", "analysis_results", "projects"} {
		var exists bool
		if err := pool.QueryRow(context.Background(),
			`select exists (
			   select 1 from information_schema.tables
			   where table_schema = 'public' and table_name = $1
			 )`, table).Scan(&exists); err != nil {
			t.Fatalf("look up %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s is missing after the migrations ran", table)
		}
	}

	// The column the whole projects migration was about (00007). Its
	// absence would make every experiments query fail in a confusing way.
	var notNull bool
	if err := pool.QueryRow(context.Background(),
		`select is_nullable = 'NO' from information_schema.columns
		 where table_name = 'experiments' and column_name = 'project_id'`).Scan(&notNull); err != nil {
		t.Fatalf("look up experiments.project_id: %v", err)
	}
	if !notNull {
		t.Error("experiments.project_id is nullable, want NOT NULL after migration 00007")
	}
}

// TestNewUser covers the piece every other test leans on: that a user
// handed out here can actually own rows, and that two calls never collide.
func TestNewUser(t *testing.T) {
	pool := pgtest.Pool(t)

	first := pgtest.NewUser(t, pool)
	second := pgtest.NewUser(t, pool)
	if first == second {
		t.Fatal("two calls returned the same user; tests sharing a database would see each other's rows")
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		`select count(*) from profiles where id = $1`, first).Scan(&count); err != nil {
		t.Fatalf("count profiles: %v", err)
	}
	if count != 1 {
		t.Errorf("profiles rows for the new user = %d, want 1", count)
	}
}

// TestNewAuthUser pins the difference from NewUser: the Supabase-side row
// only, so a test can watch EnsureProfile create the profile.
func TestNewAuthUser(t *testing.T) {
	pool := pgtest.Pool(t)
	userID := pgtest.NewAuthUser(t, pool)

	var count int
	if err := pool.QueryRow(context.Background(),
		`select count(*) from profiles where id = $1`, userID).Scan(&count); err != nil {
		t.Fatalf("count profiles: %v", err)
	}
	if count != 0 {
		t.Errorf("profiles rows = %d, want 0 until EnsureProfile runs", count)
	}
}

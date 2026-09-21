//go:build integration

// Package pgtest gives the //go:build integration tests a real Postgres
// with the production schema on it. It exists because the repositories'
// SQL -- in particular the "and user_id = $2" that is the whole of this
// app's authorization -- cannot be checked by a fake: a fake store answers
// whatever it was told to, including for a query that no longer filters by
// owner (KAN-31).
//
// The whole package is behind the integration tag, so an ordinary
// `go test ./...` neither builds nor needs it.
package pgtest

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"analyseapp/api/internal/migrations"
)

// URLEnv names the environment variable holding the throwaway database's
// connection string. Deliberately not DATABASE_URL: these tests create and
// delete rows, and pointing them at the real database by leaving one
// variable set is not a mistake worth making possible.
const URLEnv = "TEST_DATABASE_URL"

// authUsersStub is the part of Supabase's schema the migrations depend on
// but do not create: profiles.id references auth.users (id), and that table
// belongs to Supabase's own auth stack. A plain Postgres has neither, so
// migration 00001 fails against one without this.
//
// Only the column the foreign key needs is recreated. This is a stand-in
// for the tests' benefit, not a model of Supabase's table, and nothing here
// runs against a real database.
const authUsersStub = `
create schema if not exists auth;
create table if not exists auth.users (id uuid primary key);
`

var (
	migrateOnce sync.Once
	migrateErr  error
)

// Pool connects to URLEnv's database, applies the migrations once per test
// binary, and hands back a pool. Every caller shares the one database;
// tests stay independent by owning distinct users (see NewUser) rather than
// by getting a database each, which also means they exercise the same
// per-user scoping the queries rely on in production.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv(URLEnv)
	if url == "" {
		t.Fatalf("%s is not set. These tests need a throwaway Postgres, e.g.\n"+
			"  docker run --rm -d -p 5433:5432 -e POSTGRES_PASSWORD=postgres --name analyseapp-test-db postgres:16\n"+
			"  %s='postgres://postgres:postgres@localhost:5433/postgres?sslmode=disable' go test -tags=integration ./...",
			URLEnv, URLEnv)
	}

	migrateOnce.Do(func() { migrateErr = migrate(url) })
	if migrateErr != nil {
		t.Fatalf("prepare schema: %v", migrateErr)
	}

	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping %s: %v", URLEnv, err)
	}
	return pool
}

// migrate stands up the schema through the same goose path cmd/migrate
// uses, so what the tests run against is what production runs against --
// a hand-maintained copy of the DDL would drift and take the tests' value
// with it.
func migrate(url string) error {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return err
	}
	defer func() {
		// This connection exists only to run goose; the pool the tests use
		// is a separate one. A failed close here has nothing left to
		// affect, so there is nothing to report it to.
		_ = db.Close()
	}()

	if _, err := db.Exec(authUsersStub); err != nil {
		return err
	}

	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, "sql")
}

// NewAuthUser inserts a Supabase-side user and returns its id. The profiles
// row is left out on purpose, so a test can watch EnsureProfile create it.
func NewAuthUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	id := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`insert into auth.users (id) values ($1)`, id); err != nil {
		t.Fatalf("insert auth user: %v", err)
	}
	return id
}

// NewUser returns a user that can already own rows: present in auth.users
// and in profiles. Each call is a fresh uuid, which is what keeps tests
// sharing one database from seeing each other's rows.
func NewUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	id := NewAuthUser(t, pool)
	if _, err := pool.Exec(context.Background(),
		`insert into profiles (id) values ($1)`, id); err != nil {
		t.Fatalf("insert profile: %v", err)
	}
	return id
}

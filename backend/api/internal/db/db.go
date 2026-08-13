// Package db wires the Postgres connection pool used to talk to Supabase.
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a connection pool. Connections are established lazily on
// first use, so this does not itself verify reachability — callers that need
// a liveness/readiness check should Ping the pool.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, databaseURL)
}

// Package experiments implements the data-access layer for the experiments
// table described in PDR.md section 5.
package experiments

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when an experiment does not exist, or does not
// belong to the requesting user. The two cases are deliberately
// indistinguishable to callers so ownership is never leaked.
var ErrNotFound = errors.New("experiment not found")

type Experiment struct {
	ID        uuid.UUID      `json:"id"`
	UserID    uuid.UUID      `json:"user_id"`
	Title     *string        `json:"title"`
	RawData   map[string]any `json:"raw_data"`
	Config    map[string]any `json:"config"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// Store is the persistence interface httpserver's handlers depend on.
// *Repository is the real Postgres-backed implementation; tests can supply
// a fake instead so handler logic (validation, status codes) is testable
// without a database.
type Store interface {
	EnsureProfile(ctx context.Context, userID uuid.UUID) error
	Create(ctx context.Context, userID uuid.UUID, title *string, rawData, config map[string]any) (Experiment, error)
	GetByID(ctx context.Context, id, userID uuid.UUID) (Experiment, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Experiment, error)
	UpdateConfig(ctx context.Context, id, userID uuid.UUID, config map[string]any) (Experiment, error)
	UpdateRawData(ctx context.Context, id, userID uuid.UUID, rawData map[string]any) (Experiment, error)
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// queryRowExperiment runs a query expected to return exactly one experiments
// row (in the column order id, user_id, title, raw_data, config, created_at,
// updated_at) and scans it into an Experiment. A row-not-found result is
// normalized to ErrNotFound -- shared by every Repository method whose query
// is scoped to a single experiment by id/user_id.
func (r *Repository) queryRowExperiment(ctx context.Context, query string, args ...any) (Experiment, error) {
	var e Experiment
	err := r.pool.QueryRow(ctx, query, args...).
		Scan(&e.ID, &e.UserID, &e.Title, &e.RawData, &e.Config, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Experiment{}, ErrNotFound
		}
		return Experiment{}, err
	}
	return e, nil
}

// EnsureProfile creates a profiles row for userID if one doesn't already
// exist. Only needed on the create path: once an experiment exists its
// user_id FK already guarantees the profile is present.
func (r *Repository) EnsureProfile(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`insert into profiles (id) values ($1) on conflict (id) do nothing`,
		userID,
	)
	return err
}

func (r *Repository) Create(ctx context.Context, userID uuid.UUID, title *string, rawData, config map[string]any) (Experiment, error) {
	if config == nil {
		config = map[string]any{}
	}
	return r.queryRowExperiment(ctx,
		`insert into experiments (user_id, title, raw_data, config)
		 values ($1, $2, $3, $4)
		 returning id, user_id, title, raw_data, config, created_at, updated_at`,
		userID, title, rawData, config,
	)
}

func (r *Repository) GetByID(ctx context.Context, id, userID uuid.UUID) (Experiment, error) {
	return r.queryRowExperiment(ctx,
		`select id, user_id, title, raw_data, config, created_at, updated_at
		 from experiments where id = $1 and user_id = $2`,
		id, userID,
	)
}

// ListByUser returns all of userID's experiments, most recently created
// first. Always non-nil, even when there are no rows, so callers can
// serialize it directly as a JSON array.
func (r *Repository) ListByUser(ctx context.Context, userID uuid.UUID) ([]Experiment, error) {
	rows, err := r.pool.Query(ctx,
		`select id, user_id, title, raw_data, config, created_at, updated_at
		 from experiments where user_id = $1 order by created_at desc`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Experiment{}
	for rows.Next() {
		var e Experiment
		if err := rows.Scan(&e.ID, &e.UserID, &e.Title, &e.RawData, &e.Config, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

// Delete removes an experiment owned by userID. analysis_results rows for it
// are removed too, via the table's "on delete cascade" FK (see
// 00004_create_analysis_results.sql) -- no application-level cleanup needed.
func (r *Repository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`delete from experiments where id = $1 and user_id = $2`,
		id, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) UpdateConfig(ctx context.Context, id, userID uuid.UUID, config map[string]any) (Experiment, error) {
	return r.queryRowExperiment(ctx,
		`update experiments set config = $1, updated_at = now()
		 where id = $2 and user_id = $3
		 returning id, user_id, title, raw_data, config, created_at, updated_at`,
		config, id, userID,
	)
}

func (r *Repository) UpdateRawData(ctx context.Context, id, userID uuid.UUID, rawData map[string]any) (Experiment, error) {
	return r.queryRowExperiment(ctx,
		`update experiments set raw_data = $1, updated_at = now()
		 where id = $2 and user_id = $3
		 returning id, user_id, title, raw_data, config, created_at, updated_at`,
		rawData, id, userID,
	)
}

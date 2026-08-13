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
	Title     string         `json:"title"`
	RawData   map[string]any `json:"raw_data"`
	Config    map[string]any `json:"config"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
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

func (r *Repository) Create(ctx context.Context, userID uuid.UUID, title string, rawData, config map[string]any) (Experiment, error) {
	if config == nil {
		config = map[string]any{}
	}

	var e Experiment
	err := r.pool.QueryRow(ctx,
		`insert into experiments (user_id, title, raw_data, config)
		 values ($1, $2, $3, $4)
		 returning id, user_id, title, raw_data, config, created_at, updated_at`,
		userID, title, rawData, config,
	).Scan(&e.ID, &e.UserID, &e.Title, &e.RawData, &e.Config, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return Experiment{}, err
	}
	return e, nil
}

func (r *Repository) GetByID(ctx context.Context, id, userID uuid.UUID) (Experiment, error) {
	var e Experiment
	err := r.pool.QueryRow(ctx,
		`select id, user_id, title, raw_data, config, created_at, updated_at
		 from experiments where id = $1 and user_id = $2`,
		id, userID,
	).Scan(&e.ID, &e.UserID, &e.Title, &e.RawData, &e.Config, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Experiment{}, ErrNotFound
		}
		return Experiment{}, err
	}
	return e, nil
}

func (r *Repository) UpdateConfig(ctx context.Context, id, userID uuid.UUID, config map[string]any) (Experiment, error) {
	var e Experiment
	err := r.pool.QueryRow(ctx,
		`update experiments set config = $1, updated_at = now()
		 where id = $2 and user_id = $3
		 returning id, user_id, title, raw_data, config, created_at, updated_at`,
		config, id, userID,
	).Scan(&e.ID, &e.UserID, &e.Title, &e.RawData, &e.Config, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Experiment{}, ErrNotFound
		}
		return Experiment{}, err
	}
	return e, nil
}

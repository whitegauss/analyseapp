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
	ID     uuid.UUID `json:"id"`
	UserID uuid.UUID `json:"user_id"`
	// ProjectID is NOT NULL in the database (PDR.md section 5): every
	// experiment belongs to exactly one project. UserID stays alongside it
	// even though it is reachable through the project, so authorization
	// remains a single-table "where id = $1 and user_id = $2" check.
	ProjectID uuid.UUID      `json:"project_id"`
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
	Create(ctx context.Context, userID, projectID uuid.UUID, title *string, rawData, config map[string]any) (Experiment, error)
	GetByID(ctx context.Context, id, userID uuid.UUID) (Experiment, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Experiment, error)
	ListByProject(ctx context.Context, projectID, userID uuid.UUID) ([]Experiment, error)
	Copy(ctx context.Context, id, userID, targetProjectID uuid.UUID) (Experiment, error)
	UpdateProject(ctx context.Context, id, userID, targetProjectID uuid.UUID) (Experiment, error)
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
// row (in the column order id, user_id, project_id, title, raw_data, config,
// created_at, updated_at) and scans it into an Experiment. A row-not-found result is
// normalized to ErrNotFound -- shared by every Repository method whose query
// is scoped to a single experiment by id/user_id.
func (r *Repository) queryRowExperiment(ctx context.Context, query string, args ...any) (Experiment, error) {
	var e Experiment
	err := r.pool.QueryRow(ctx, query, args...).
		Scan(&e.ID, &e.UserID, &e.ProjectID, &e.Title, &e.RawData, &e.Config, &e.CreatedAt, &e.UpdatedAt)
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

// Create stores an experiment in projectID. The insert is guarded by the
// project's ownership rather than trusting the caller: an id belonging to
// someone else (or to nothing at all) selects no row, so the insert writes
// nothing and this reports ErrNotFound. Callers turn that into the same 404
// an unknown experiment id gets, which is what keeps another user's project
// ids from being probed through this endpoint.
func (r *Repository) Create(ctx context.Context, userID, projectID uuid.UUID, title *string, rawData, config map[string]any) (Experiment, error) {
	if config == nil {
		config = map[string]any{}
	}
	return r.queryRowExperiment(ctx,
		`insert into experiments (user_id, project_id, title, raw_data, config)
		 select $1, $2, $3, $4, $5
		 where exists (select 1 from projects where id = $2 and user_id = $1)
		 returning id, user_id, project_id, title, raw_data, config, created_at, updated_at`,
		userID, projectID, title, rawData, config,
	)
}

func (r *Repository) GetByID(ctx context.Context, id, userID uuid.UUID) (Experiment, error) {
	return r.queryRowExperiment(ctx,
		`select id, user_id, project_id, title, raw_data, config, created_at, updated_at
		 from experiments where id = $1 and user_id = $2`,
		id, userID,
	)
}

// queryExperiments runs a query returning any number of experiments rows (in
// the same column order queryRowExperiment expects) and scans them all. The
// result is always non-nil, even when there are no rows, so callers can
// serialize it directly as a JSON array rather than as null.
func (r *Repository) queryExperiments(ctx context.Context, query string, args ...any) ([]Experiment, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Experiment{}
	for rows.Next() {
		var e Experiment
		if err := rows.Scan(&e.ID, &e.UserID, &e.ProjectID, &e.Title, &e.RawData, &e.Config, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

// ListByUser returns all of userID's experiments across every project, most
// recently created first. The cross-project view stays (PDR.md section 8):
// picking a copy source and comparing experiments both need it.
func (r *Repository) ListByUser(ctx context.Context, userID uuid.UUID) ([]Experiment, error) {
	return r.queryExperiments(ctx,
		`select id, user_id, project_id, title, raw_data, config, created_at, updated_at
		 from experiments where user_id = $1 order by created_at desc`,
		userID,
	)
}

// ListByProject returns the experiments in one project, most recently
// created first. userID is part of the filter rather than a separate
// ownership lookup, so another user's rows can never come back even if
// projectID somehow names their project.
//
// An empty result says nothing about whether the project exists: callers
// that need to tell "no experiments yet" from "no such project" check the
// project itself first (handleListProjectExperiments does).
func (r *Repository) ListByProject(ctx context.Context, projectID, userID uuid.UUID) ([]Experiment, error) {
	return r.queryExperiments(ctx,
		`select id, user_id, project_id, title, raw_data, config, created_at, updated_at
		 from experiments where project_id = $1 and user_id = $2 order by created_at desc`,
		projectID, userID,
	)
}

// Copy duplicates an experiment into targetProjectID as a new row with a
// new id (PDR.md section 5: another project's data is taken in by copying,
// not by reference, so the two are independent from then on). raw_data and
// config are copied as they are; created_at and updated_at take their
// defaults, because the copy is a new experiment rather than a record of
// the original's age.
//
// One statement, so nothing can change underneath it: the select supplies
// the row only if the source is userID's, and the exists clause only if the
// destination project is too. Either one failing writes nothing and reports
// ErrNotFound. Handlers check the destination separately beforehand to say
// which of the two was wrong -- the guard here is what makes that check
// safe to race with a delete.
//
// No cache invalidation (PDR.md section 7): the copy has a new experiment
// id, so it shares no analysis:{experiment_id}:* key with its source, and
// the source's own results are still correct.
func (r *Repository) Copy(ctx context.Context, id, userID, targetProjectID uuid.UUID) (Experiment, error) {
	return r.queryRowExperiment(ctx,
		`insert into experiments (user_id, project_id, title, raw_data, config)
		 select e.user_id, $3, e.title, e.raw_data, e.config
		 from experiments e
		 where e.id = $1 and e.user_id = $2
		   and exists (select 1 from projects p where p.id = $3 and p.user_id = $2)
		 returning id, user_id, project_id, title, raw_data, config, created_at, updated_at`,
		id, userID, targetProjectID,
	)
}

// UpdateProject moves an experiment to targetProjectID, keeping its id and
// everything else. This is the counterpart to Copy: Copy duplicates and
// leaves the source alone, while this re-parents the row in place, so
// existing links to /experiments/{id} keep working and created_at still
// says when the measurement was taken.
//
// Guarded the same way as Copy, in one statement: the where clause requires
// the experiment to be userID's and the destination project to be as well,
// so either one failing updates nothing and reports ErrNotFound.
//
// Moving an experiment to the project it is already in is allowed and
// succeeds unchanged -- the row still matches, so it comes back as a plain
// 200. There is nothing to protect against there, and making it an error
// would only force callers to compare ids first.
//
// No cache invalidation (PDR.md section 7): analysis:{experiment_id}:* does
// not include the project, and raw_data has not changed, so every cached
// result for this experiment is still correct.
func (r *Repository) UpdateProject(ctx context.Context, id, userID, targetProjectID uuid.UUID) (Experiment, error) {
	return r.queryRowExperiment(ctx,
		`update experiments e
		 set project_id = $3, updated_at = now()
		 where e.id = $1 and e.user_id = $2
		   and exists (select 1 from projects p where p.id = $3 and p.user_id = $2)
		 returning e.id, e.user_id, e.project_id, e.title, e.raw_data, e.config, e.created_at, e.updated_at`,
		id, userID, targetProjectID,
	)
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
		 returning id, user_id, project_id, title, raw_data, config, created_at, updated_at`,
		config, id, userID,
	)
}

func (r *Repository) UpdateRawData(ctx context.Context, id, userID uuid.UUID, rawData map[string]any) (Experiment, error) {
	return r.queryRowExperiment(ctx,
		`update experiments set raw_data = $1, updated_at = now()
		 where id = $2 and user_id = $3
		 returning id, user_id, project_id, title, raw_data, config, created_at, updated_at`,
		rawData, id, userID,
	)
}

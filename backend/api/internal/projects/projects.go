// Package projects implements the data-access layer for the projects
// table described in PDR.md section 5. A project groups a user's
// experiments; every experiment belongs to exactly one project (see
// experiments.Experiment.ProjectID).
package projects

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a project does not exist, or does not belong
// to the requesting user. The two cases are deliberately indistinguishable
// to callers so ownership is never leaked.
var ErrNotFound = errors.New("project not found")

// DefaultTitle is the project every user gets implicitly: experiments
// created without naming a project land here, and the 00007 migration moved
// every pre-projects experiment into one of these. A partial unique index
// (projects_user_default_idx) keeps it to one per user, so the migration and
// EnsureDefault always mean the same project.
const DefaultTitle = "未分類"

const defaultDescription = "プロジェクトを選ばずに作成した実験の置き場所です。"

type Project struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Store is the persistence interface httpserver's handlers depend on.
// *Repository is the real Postgres-backed implementation; tests can supply
// a fake instead so handler logic (validation, status codes) is testable
// without a database.
type Store interface {
	EnsureProfile(ctx context.Context, userID uuid.UUID) error
	Create(ctx context.Context, userID uuid.UUID, title, description string) (Project, error)
	EnsureDefault(ctx context.Context, userID uuid.UUID) (Project, error)
	GetByID(ctx context.Context, id, userID uuid.UUID) (Project, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]Project, error)
	Update(ctx context.Context, id, userID uuid.UUID, title, description string) (Project, error)
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// queryRowProject runs a query expected to return exactly one projects row
// (in the column order id, user_id, title, description, created_at,
// updated_at) and scans it into a Project. A row-not-found result is
// normalized to ErrNotFound -- shared by every Repository method whose
// query is scoped to a single project by id/user_id.
func (r *Repository) queryRowProject(ctx context.Context, query string, args ...any) (Project, error) {
	var p Project
	err := r.pool.QueryRow(ctx, query, args...).
		Scan(&p.ID, &p.UserID, &p.Title, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Project{}, ErrNotFound
		}
		return Project{}, err
	}
	return p, nil
}

// EnsureProfile creates a profiles row for userID if one doesn't already
// exist. Only needed on the create path: once a project exists its user_id
// FK already guarantees the profile is present. Duplicated from
// experiments.Repository.EnsureProfile (same one-line upsert against the
// shared profiles table) rather than factored into a shared helper -- both
// repositories are independent, pool-backed, and this is the entire method.
func (r *Repository) EnsureProfile(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`insert into profiles (id) values ($1) on conflict (id) do nothing`,
		userID,
	)
	return err
}

func (r *Repository) Create(ctx context.Context, userID uuid.UUID, title, description string) (Project, error) {
	return r.queryRowProject(ctx,
		`insert into projects (user_id, title, description)
		 values ($1, $2, $3)
		 returning id, user_id, title, description, created_at, updated_at`,
		userID, title, description,
	)
}

// EnsureDefault returns the user's DefaultTitle project, creating it on
// first use. This is what keeps POST /api/v1/experiments (which names no
// project) working now that experiments.project_id is NOT NULL.
//
// Two round trips instead of one upsert, because an upsert that returns the
// existing row needs ON CONFLICT inference to name the partial index's
// predicate literally, which is a good deal harder to read than this. The
// insert still carries ON CONFLICT DO NOTHING so that two concurrent first
// requests cannot create two default projects: the loser gets no row back
// and reads the winner's instead.
func (r *Repository) EnsureDefault(ctx context.Context, userID uuid.UUID) (Project, error) {
	p, err := r.getDefault(ctx, userID)
	if err == nil || !errors.Is(err, ErrNotFound) {
		return p, err
	}

	p, err = r.queryRowProject(ctx,
		`insert into projects (user_id, title, description)
		 values ($1, $2, $3)
		 on conflict do nothing
		 returning id, user_id, title, description, created_at, updated_at`,
		userID, DefaultTitle, defaultDescription,
	)
	if errors.Is(err, ErrNotFound) {
		return r.getDefault(ctx, userID)
	}
	return p, err
}

// getDefault reads the user's default project. Ordered and limited rather
// than assuming uniqueness: the partial unique index only covers rows
// created after the 00007 migration ran, and a database that predates it
// could hold two.
func (r *Repository) getDefault(ctx context.Context, userID uuid.UUID) (Project, error) {
	return r.queryRowProject(ctx,
		`select id, user_id, title, description, created_at, updated_at
		 from projects where user_id = $1 and title = $2
		 order by created_at, id limit 1`,
		userID, DefaultTitle,
	)
}

func (r *Repository) GetByID(ctx context.Context, id, userID uuid.UUID) (Project, error) {
	return r.queryRowProject(ctx,
		`select id, user_id, title, description, created_at, updated_at
		 from projects where id = $1 and user_id = $2`,
		id, userID,
	)
}

// ListByUser returns all of userID's projects, most recently created first.
// Always non-nil, even when there are no rows, so callers can serialize it
// directly as a JSON array.
func (r *Repository) ListByUser(ctx context.Context, userID uuid.UUID) ([]Project, error) {
	rows, err := r.pool.Query(ctx,
		`select id, user_id, title, description, created_at, updated_at
		 from projects where user_id = $1 order by created_at desc`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.UserID, &p.Title, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *Repository) Update(ctx context.Context, id, userID uuid.UUID, title, description string) (Project, error) {
	return r.queryRowProject(ctx,
		`update projects set title = $1, description = $2, updated_at = now()
		 where id = $3 and user_id = $4
		 returning id, user_id, title, description, created_at, updated_at`,
		title, description, id, userID,
	)
}

// Delete removes a project owned by userID. Experiments belonging to it are
// removed too, via experiments.project_id's "on delete cascade" FK (see
// 00007_experiments_add_project_id.sql) -- no application-level cleanup
// needed, but the UI has to say so before asking for confirmation.
func (r *Repository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`delete from projects where id = $1 and user_id = $2`,
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

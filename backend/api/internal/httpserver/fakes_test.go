package httpserver

import (
	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/projects"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Fakes for the handlers' dependencies (the experiments/projects stores,
// the worker client, the result cache), shared by every test file in the
// package. Kept apart from any one handler's tests so that deleting or
// rewriting those tests cannot break the others (KAN-38).

// fakeStore is a minimal experiments.Store implementation for handler
// tests, so handler validation/status-code logic is testable without a
// database. Unset function fields fail the test if called.
type fakeStore struct {
	t *testing.T
	// ensureProfileFn is optional, unlike the rest: nil means EnsureProfile
	// succeeds, which is what every handler test but the create path's
	// needs.
	ensureProfileFn func(ctx context.Context, userID uuid.UUID) error
	createFn        func(ctx context.Context, userID, projectID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error)
	getByIDFn       func(ctx context.Context, id, userID uuid.UUID) (experiments.Experiment, error)
	listByUserFn    func(ctx context.Context, userID uuid.UUID) ([]experiments.Experiment, error)
	listByProjectFn func(ctx context.Context, projectID, userID uuid.UUID) ([]experiments.Experiment, error)
	copyFn          func(ctx context.Context, id, userID, targetProjectID uuid.UUID) (experiments.Experiment, error)
	updateProjectFn func(ctx context.Context, id, userID, targetProjectID uuid.UUID) (experiments.Experiment, error)
	updateConfigFn  func(ctx context.Context, id, userID uuid.UUID, config map[string]any) (experiments.Experiment, error)
	updateRawDataFn func(ctx context.Context, id, userID uuid.UUID, rawData map[string]any) (experiments.Experiment, error)
	deleteFn        func(ctx context.Context, id, userID uuid.UUID) error
}

func (f *fakeStore) EnsureProfile(ctx context.Context, userID uuid.UUID) error {
	if f.ensureProfileFn == nil {
		return nil
	}
	return f.ensureProfileFn(ctx, userID)
}

func (f *fakeStore) Create(ctx context.Context, userID, projectID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error) {
	if f.createFn == nil {
		f.t.Fatal("unexpected call to Create")
	}
	return f.createFn(ctx, userID, projectID, title, rawData, config)
}

func (f *fakeStore) GetByID(ctx context.Context, id, userID uuid.UUID) (experiments.Experiment, error) {
	if f.getByIDFn == nil {
		f.t.Fatal("unexpected call to GetByID")
	}
	return f.getByIDFn(ctx, id, userID)
}

func (f *fakeStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]experiments.Experiment, error) {
	if f.listByUserFn == nil {
		f.t.Fatal("unexpected call to ListByUser")
	}
	return f.listByUserFn(ctx, userID)
}

func (f *fakeStore) ListByProject(ctx context.Context, projectID, userID uuid.UUID) ([]experiments.Experiment, error) {
	if f.listByProjectFn == nil {
		f.t.Fatal("unexpected call to ListByProject")
	}
	return f.listByProjectFn(ctx, projectID, userID)
}

func (f *fakeStore) Copy(ctx context.Context, id, userID, targetProjectID uuid.UUID) (experiments.Experiment, error) {
	if f.copyFn == nil {
		f.t.Fatal("unexpected call to Copy")
	}
	return f.copyFn(ctx, id, userID, targetProjectID)
}

func (f *fakeStore) UpdateProject(ctx context.Context, id, userID, targetProjectID uuid.UUID) (experiments.Experiment, error) {
	if f.updateProjectFn == nil {
		f.t.Fatal("unexpected call to UpdateProject")
	}
	return f.updateProjectFn(ctx, id, userID, targetProjectID)
}

func (f *fakeStore) UpdateConfig(ctx context.Context, id, userID uuid.UUID, config map[string]any) (experiments.Experiment, error) {
	if f.updateConfigFn == nil {
		f.t.Fatal("unexpected call to UpdateConfig")
	}
	return f.updateConfigFn(ctx, id, userID, config)
}

func (f *fakeStore) UpdateRawData(ctx context.Context, id, userID uuid.UUID, rawData map[string]any) (experiments.Experiment, error) {
	if f.updateRawDataFn == nil {
		f.t.Fatal("unexpected call to UpdateRawData")
	}
	return f.updateRawDataFn(ctx, id, userID, rawData)
}

func (f *fakeStore) Delete(ctx context.Context, id, userID uuid.UUID) error {
	if f.deleteFn == nil {
		f.t.Fatal("unexpected call to Delete")
	}
	return f.deleteFn(ctx, id, userID)
}

// fakeProjectStore is a minimal projects.Store implementation for handler
// tests, so handler validation/status-code logic is testable without a
// database. Unset function fields fail the test if called.
type fakeProjectStore struct {
	t               *testing.T
	createFn        func(ctx context.Context, userID uuid.UUID, title, description string) (projects.Project, error)
	ensureDefaultFn func(ctx context.Context, userID uuid.UUID) (projects.Project, error)
	getByIDFn       func(ctx context.Context, id, userID uuid.UUID) (projects.Project, error)
	listByUserFn    func(ctx context.Context, userID uuid.UUID) ([]projects.Project, error)
	updateFn        func(ctx context.Context, id, userID uuid.UUID, title, description string) (projects.Project, error)
	deleteFn        func(ctx context.Context, id, userID uuid.UUID) error
}

func (f *fakeProjectStore) EnsureProfile(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (f *fakeProjectStore) Create(ctx context.Context, userID uuid.UUID, title, description string) (projects.Project, error) {
	if f.createFn == nil {
		f.t.Fatal("unexpected call to Create")
	}
	return f.createFn(ctx, userID, title, description)
}

func (f *fakeProjectStore) EnsureDefault(ctx context.Context, userID uuid.UUID) (projects.Project, error) {
	if f.ensureDefaultFn == nil {
		f.t.Fatal("unexpected call to EnsureDefault")
	}
	return f.ensureDefaultFn(ctx, userID)
}

// defaultProjectStore is the projects.Store the flat create path needs: it
// answers EnsureDefault with one fixed project and refuses every other call.
func defaultProjectStore(t *testing.T, project projects.Project) *fakeProjectStore {
	t.Helper()
	return &fakeProjectStore{
		t: t,
		ensureDefaultFn: func(ctx context.Context, userID uuid.UUID) (projects.Project, error) {
			project.UserID = userID
			return project, nil
		},
	}
}

func (f *fakeProjectStore) GetByID(ctx context.Context, id, userID uuid.UUID) (projects.Project, error) {
	if f.getByIDFn == nil {
		f.t.Fatal("unexpected call to GetByID")
	}
	return f.getByIDFn(ctx, id, userID)
}

func (f *fakeProjectStore) ListByUser(ctx context.Context, userID uuid.UUID) ([]projects.Project, error) {
	if f.listByUserFn == nil {
		f.t.Fatal("unexpected call to ListByUser")
	}
	return f.listByUserFn(ctx, userID)
}

func (f *fakeProjectStore) Update(ctx context.Context, id, userID uuid.UUID, title, description string) (projects.Project, error) {
	if f.updateFn == nil {
		f.t.Fatal("unexpected call to Update")
	}
	return f.updateFn(ctx, id, userID, title, description)
}

func (f *fakeProjectStore) Delete(ctx context.Context, id, userID uuid.UUID) error {
	if f.deleteFn == nil {
		f.t.Fatal("unexpected call to Delete")
	}
	return f.deleteFn(ctx, id, userID)
}

// fakeWorkerClient is a minimal worker.Client implementation for handler
// tests, so handler logic is testable without a running Python worker.
type fakeWorkerClient struct {
	t         *testing.T
	analyzeFn func(ctx context.Context, traceID string, body []byte) (int, []byte, error)
}

func (f *fakeWorkerClient) Analyze(ctx context.Context, traceID string, body []byte) (int, []byte, error) {
	if f.analyzeFn == nil {
		f.t.Fatal("unexpected call to Analyze")
	}
	return f.analyzeFn(ctx, traceID, body)
}

// fakeCache is an in-memory cache.Cache implementation for handler tests.
type fakeCache struct {
	store map[string][]byte
}

func newFakeCache() *fakeCache {
	return &fakeCache{store: map[string][]byte{}}
}

func (c *fakeCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	v, ok := c.store[key]
	return v, ok, nil
}

func (c *fakeCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	c.store[key] = value
	return nil
}

func (c *fakeCache) DeleteByPrefix(ctx context.Context, prefix string) error {
	for key := range c.store {
		if strings.HasPrefix(key, prefix) {
			delete(c.store, key)
		}
	}
	return nil
}

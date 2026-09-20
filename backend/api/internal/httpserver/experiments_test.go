package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"analyseapp/api/internal/auth"
	"analyseapp/api/internal/cache"
	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/projects"
	"analyseapp/api/internal/response"
)

var testUserID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

// fakeStore is a minimal experiments.Store implementation for handler
// tests, so handler validation/status-code logic is testable without a
// database. Unset function fields fail the test if called.
type fakeStore struct {
	t               *testing.T
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
	return nil
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

// newTestRequest builds a request carrying a chi "id" URL param and,
// optionally, an authenticated user in context (mirroring what auth.Middleware
// would have set).
func newTestRequest(method, id, body string, authenticated bool) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, "/", strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, "/", nil)
	}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	if authenticated {
		ctx = auth.WithUserID(ctx, testUserID)
	}
	return r.WithContext(ctx)
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) response.Envelope {
	t.Helper()
	var body response.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v (body=%s)", err, rec.Body.String())
	}
	return body
}

func TestHandleCreateExperiment(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("POST", "", `{"raw_data":{}}`, false)
		rec := httptest.NewRecorder()

		handleCreateExperiment(store, defaultProjectStore(t, projects.Project{ID: uuid.New()}))(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("POST", "", `not json`, true)
		rec := httptest.NewRecorder()

		handleCreateExperiment(store, defaultProjectStore(t, projects.Project{ID: uuid.New()}))(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_body" {
			t.Errorf("error code = %+v, want invalid_body", body.Error)
		}
	})

	t.Run("missing raw_data", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("POST", "", `{"title":"x"}`, true)
		rec := httptest.NewRecorder()

		handleCreateExperiment(store, defaultProjectStore(t, projects.Project{ID: uuid.New()}))(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_raw_data" {
			t.Errorf("error code = %+v, want invalid_raw_data", body.Error)
		}
	})

	t.Run("empty title is normalized to nil", func(t *testing.T) {
		var gotTitle *string
		gotTitleSet := false
		store := &fakeStore{
			t: t,
			createFn: func(ctx context.Context, userID, projectID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error) {
				gotTitle = title
				gotTitleSet = true
				return experiments.Experiment{ID: uuid.New(), UserID: userID, ProjectID: projectID, Title: title, RawData: rawData, Config: config}, nil
			},
		}
		req := newTestRequest("POST", "", `{"title":"","raw_data":{"columns":{}}}`, true)
		rec := httptest.NewRecorder()

		handleCreateExperiment(store, defaultProjectStore(t, projects.Project{ID: uuid.New()}))(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
		}
		if !gotTitleSet {
			t.Fatal("Create was not called")
		}
		if gotTitle != nil {
			t.Errorf("title passed to store = %v, want nil", *gotTitle)
		}
	})

	t.Run("stores the experiment in the user's default project", func(t *testing.T) {
		// The flat path names no project, so the handler has to supply one:
		// without it the insert violates experiments.project_id's NOT NULL.
		defaultProject := projects.Project{ID: uuid.New(), Title: projects.DefaultTitle}
		var gotProjectID uuid.UUID
		store := &fakeStore{
			t: t,
			createFn: func(ctx context.Context, userID, projectID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error) {
				gotProjectID = projectID
				return experiments.Experiment{ID: uuid.New(), UserID: userID, ProjectID: projectID, RawData: rawData}, nil
			},
		}
		req := newTestRequest("POST", "", `{"raw_data":{"columns":{"x":[1]}}}`, true)
		rec := httptest.NewRecorder()

		handleCreateExperiment(store, defaultProjectStore(t, defaultProject))(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
		}
		if gotProjectID != defaultProject.ID {
			t.Errorf("project id passed to store = %v, want the default project %v", gotProjectID, defaultProject.ID)
		}
		if !strings.Contains(rec.Body.String(), defaultProject.ID.String()) {
			t.Errorf("body = %s, want it to carry project_id %v", rec.Body.String(), defaultProject.ID)
		}
	})

	t.Run("default project cannot be resolved", func(t *testing.T) {
		// Nothing can be stored without a project, so this is a 500 rather
		// than an experiment quietly created somewhere else.
		store := &fakeStore{t: t}
		projectStore := &fakeProjectStore{
			t: t,
			ensureDefaultFn: func(ctx context.Context, userID uuid.UUID) (projects.Project, error) {
				return projects.Project{}, errors.New("db is down")
			},
		}
		req := newTestRequest("POST", "", `{"raw_data":{"columns":{"x":[1]}}}`, true)
		rec := httptest.NewRecorder()

		handleCreateExperiment(store, projectStore)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		title := "my experiment"
		store := &fakeStore{
			t: t,
			createFn: func(ctx context.Context, userID, projectID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error) {
				return experiments.Experiment{ID: uuid.New(), UserID: userID, ProjectID: projectID, Title: title, RawData: rawData, Config: config}, nil
			},
		}
		req := newTestRequest("POST", "", `{"title":"`+title+`","raw_data":{"columns":{"x":[1]}}}`, true)
		rec := httptest.NewRecorder()

		handleCreateExperiment(store, defaultProjectStore(t, projects.Project{ID: uuid.New()}))(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleCreateProjectExperiment(t *testing.T) {
	projectID := uuid.New()

	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("POST", projectID.String(), `{"raw_data":{}}`, false)
		rec := httptest.NewRecorder()

		handleCreateProjectExperiment(store)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid project id", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("POST", "not-a-uuid", `{"raw_data":{"columns":{}}}`, true)
		rec := httptest.NewRecorder()

		handleCreateProjectExperiment(store)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_id" {
			t.Errorf("error code = %+v, want invalid_id", body.Error)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("POST", projectID.String(), `not json`, true)
		rec := httptest.NewRecorder()

		handleCreateProjectExperiment(store)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_body" {
			t.Errorf("error code = %+v, want invalid_body", body.Error)
		}
	})

	t.Run("missing raw_data", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("POST", projectID.String(), `{"title":"x"}`, true)
		rec := httptest.NewRecorder()

		handleCreateProjectExperiment(store)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_raw_data" {
			t.Errorf("error code = %+v, want invalid_raw_data", body.Error)
		}
	})

	t.Run("empty title is normalized to nil", func(t *testing.T) {
		var gotTitle *string
		gotTitleSet := false
		store := &fakeStore{
			t: t,
			createFn: func(ctx context.Context, userID, projectID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error) {
				gotTitle = title
				gotTitleSet = true
				return experiments.Experiment{ID: uuid.New(), UserID: userID, ProjectID: projectID, Title: title, RawData: rawData}, nil
			},
		}
		req := newTestRequest("POST", projectID.String(), `{"title":"","raw_data":{"columns":{}}}`, true)
		rec := httptest.NewRecorder()

		handleCreateProjectExperiment(store)(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
		}
		if !gotTitleSet {
			t.Fatal("Create was not called")
		}
		if gotTitle != nil {
			t.Errorf("title passed to store = %v, want nil", *gotTitle)
		}
	})

	// Another user's project id, and an id that names nothing, are the same
	// case: the store's insert is guarded by ownership, so both write
	// nothing and come back as ErrNotFound. What matters is that the reply
	// is the 404 GET /api/v1/projects/{id} gives -- otherwise this endpoint
	// would confirm which project ids exist.
	t.Run("another user's project is a 404 naming the project", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			createFn: func(ctx context.Context, userID, projectID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error) {
				return experiments.Experiment{}, experiments.ErrNotFound
			},
		}
		req := newTestRequest("POST", uuid.New().String(), `{"raw_data":{"columns":{}}}`, true)
		rec := httptest.NewRecorder()

		handleCreateProjectExperiment(store)(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body=%s)", rec.Code, rec.Body.String())
		}
		body := decodeEnvelope(t, rec)
		if body.Error == nil || body.Error.Message != "project not found" {
			t.Errorf("error = %+v, want the message to name the project", body.Error)
		}
	})

	t.Run("store failure", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			createFn: func(ctx context.Context, userID, projectID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error) {
				return experiments.Experiment{}, errors.New("db is down")
			},
		}
		req := newTestRequest("POST", projectID.String(), `{"raw_data":{"columns":{}}}`, true)
		rec := httptest.NewRecorder()

		handleCreateProjectExperiment(store)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	t.Run("stores the experiment in the project from the URL", func(t *testing.T) {
		var gotProjectID, gotUserID uuid.UUID
		store := &fakeStore{
			t: t,
			createFn: func(ctx context.Context, userID, projectID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error) {
				gotProjectID = projectID
				gotUserID = userID
				return experiments.Experiment{ID: uuid.New(), UserID: userID, ProjectID: projectID, Title: title, RawData: rawData}, nil
			},
		}
		req := newTestRequest("POST", projectID.String(), `{"title":"落下運動","raw_data":{"columns":{"x":[1]}}}`, true)
		rec := httptest.NewRecorder()

		handleCreateProjectExperiment(store)(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
		}
		if gotProjectID != projectID {
			t.Errorf("project id passed to store = %v, want the one from the URL %v", gotProjectID, projectID)
		}
		if gotUserID != testUserID {
			t.Errorf("userID passed to store = %v, want %v", gotUserID, testUserID)
		}
		if !strings.Contains(rec.Body.String(), projectID.String()) {
			t.Errorf("body = %s, want it to carry project_id %v", rec.Body.String(), projectID)
		}
	})
}

func TestHandleListProjectExperiments(t *testing.T) {
	projectID := uuid.New()

	// ownedProjectStore answers GetByID with the project, as it would for
	// its owner. The list handler reads nothing else off it.
	ownedProjectStore := func(t *testing.T) *fakeProjectStore {
		return &fakeProjectStore{
			t: t,
			getByIDFn: func(ctx context.Context, id, userID uuid.UUID) (projects.Project, error) {
				return projects.Project{ID: id, UserID: userID}, nil
			},
		}
	}

	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("GET", projectID.String(), "", false)
		rec := httptest.NewRecorder()

		handleListProjectExperiments(store, &fakeProjectStore{t: t})(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid project id", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("GET", "not-a-uuid", "", true)
		rec := httptest.NewRecorder()

		handleListProjectExperiments(store, &fakeProjectStore{t: t})(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_id" {
			t.Errorf("error code = %+v, want invalid_id", body.Error)
		}
	})

	// The whole reason the handler reads the project first. Both fakes fail
	// the test if ListByProject is reached, so a 404 here also proves no
	// query ran against someone else's project id.
	t.Run("another user's project is a 404", func(t *testing.T) {
		store := &fakeStore{t: t}
		projectStore := &fakeProjectStore{
			t: t,
			getByIDFn: func(ctx context.Context, id, userID uuid.UUID) (projects.Project, error) {
				return projects.Project{}, projects.ErrNotFound
			},
		}
		req := newTestRequest("GET", uuid.New().String(), "", true)
		rec := httptest.NewRecorder()

		handleListProjectExperiments(store, projectStore)(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body=%s)", rec.Code, rec.Body.String())
		}
		body := decodeEnvelope(t, rec)
		if body.Error == nil || body.Error.Message != "project not found" {
			t.Errorf("error = %+v, want the message to name the project", body.Error)
		}
	})

	t.Run("project lookup failure", func(t *testing.T) {
		store := &fakeStore{t: t}
		projectStore := &fakeProjectStore{
			t: t,
			getByIDFn: func(ctx context.Context, id, userID uuid.UUID) (projects.Project, error) {
				return projects.Project{}, errors.New("db is down")
			},
		}
		req := newTestRequest("GET", projectID.String(), "", true)
		rec := httptest.NewRecorder()

		handleListProjectExperiments(store, projectStore)(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	// A project of the user's own that holds nothing answers 200 with an
	// empty array -- the case the 404 above must stay distinguishable from.
	t.Run("an empty project is an empty array, not a 404", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			listByProjectFn: func(ctx context.Context, projectID, userID uuid.UUID) ([]experiments.Experiment, error) {
				return []experiments.Experiment{}, nil
			},
		}
		req := newTestRequest("GET", projectID.String(), "", true)
		rec := httptest.NewRecorder()

		handleListProjectExperiments(store, ownedProjectStore(t))(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		body := decodeEnvelope(t, rec)
		list, ok := body.Data.([]any)
		if !ok || len(list) != 0 {
			t.Errorf("data = %+v, want an empty array", body.Data)
		}
	})

	t.Run("list failure", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			listByProjectFn: func(ctx context.Context, projectID, userID uuid.UUID) ([]experiments.Experiment, error) {
				return nil, errors.New("db is down")
			},
		}
		req := newTestRequest("GET", projectID.String(), "", true)
		rec := httptest.NewRecorder()

		handleListProjectExperiments(store, ownedProjectStore(t))(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	t.Run("success scopes the query to the project and the caller", func(t *testing.T) {
		var gotProjectID, gotUserID uuid.UUID
		store := &fakeStore{
			t: t,
			listByProjectFn: func(ctx context.Context, projectID, userID uuid.UUID) ([]experiments.Experiment, error) {
				gotProjectID = projectID
				gotUserID = userID
				return []experiments.Experiment{
					{ID: uuid.New(), UserID: userID, ProjectID: projectID},
					{ID: uuid.New(), UserID: userID, ProjectID: projectID},
				}, nil
			},
		}
		req := newTestRequest("GET", projectID.String(), "", true)
		rec := httptest.NewRecorder()

		handleListProjectExperiments(store, ownedProjectStore(t))(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		if gotProjectID != projectID {
			t.Errorf("project id passed to store = %v, want %v", gotProjectID, projectID)
		}
		if gotUserID != testUserID {
			t.Errorf("userID passed to store = %v, want %v", gotUserID, testUserID)
		}
		body := decodeEnvelope(t, rec)
		list, ok := body.Data.([]any)
		if !ok || len(list) != 2 {
			t.Errorf("data = %+v, want 2 experiments", body.Data)
		}
	})
}

func TestHandleCopyExperiment(t *testing.T) {
	sourceID := uuid.New()
	targetProjectID := uuid.New()
	body := `{"project_id":"` + targetProjectID.String() + `"}`

	// ownedProjectStore answers GetByID with the project, as it would for
	// its owner -- the destination check passing, so the test reaches Copy.
	ownedProjectStore := func(t *testing.T) *fakeProjectStore {
		return &fakeProjectStore{
			t: t,
			getByIDFn: func(ctx context.Context, id, userID uuid.UUID) (projects.Project, error) {
				return projects.Project{ID: id, UserID: userID}, nil
			},
		}
	}

	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("POST", sourceID.String(), body, false)
		rec := httptest.NewRecorder()

		handleCopyExperiment(store, &fakeProjectStore{t: t})(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid experiment id", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("POST", "not-a-uuid", body, true)
		rec := httptest.NewRecorder()

		handleCopyExperiment(store, &fakeProjectStore{t: t})(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if b := decodeEnvelope(t, rec); b.Error == nil || b.Error.Code != "invalid_id" {
			t.Errorf("error code = %+v, want invalid_id", b.Error)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("POST", sourceID.String(), `not json`, true)
		rec := httptest.NewRecorder()

		handleCopyExperiment(store, &fakeProjectStore{t: t})(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if b := decodeEnvelope(t, rec); b.Error == nil || b.Error.Code != "invalid_body" {
			t.Errorf("error code = %+v, want invalid_body", b.Error)
		}
	})

	// A copy has to name where it is going: without project_id there is no
	// destination to default to, since the source's own project would make
	// the call a no-op duplicate rather than what was asked for.
	t.Run("missing project_id", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("POST", sourceID.String(), `{}`, true)
		rec := httptest.NewRecorder()

		handleCopyExperiment(store, &fakeProjectStore{t: t})(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if b := decodeEnvelope(t, rec); b.Error == nil || b.Error.Code != "invalid_project_id" {
			t.Errorf("error code = %+v, want invalid_project_id", b.Error)
		}
	})

	// A malformed destination is the caller's mistake, like a malformed
	// path id: 400 rather than the 404 a well-formed unknown id gets.
	t.Run("malformed project_id", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("POST", sourceID.String(), `{"project_id":"not-a-uuid"}`, true)
		rec := httptest.NewRecorder()

		handleCopyExperiment(store, &fakeProjectStore{t: t})(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if b := decodeEnvelope(t, rec); b.Error == nil || b.Error.Code != "invalid_project_id" {
			t.Errorf("error code = %+v, want invalid_project_id", b.Error)
		}
	})

	// Both fakes fail the test if Copy is reached, so this also proves
	// nothing is written when the destination is not the caller's.
	t.Run("another user's destination project is a 404", func(t *testing.T) {
		store := &fakeStore{t: t}
		projectStore := &fakeProjectStore{
			t: t,
			getByIDFn: func(ctx context.Context, id, userID uuid.UUID) (projects.Project, error) {
				return projects.Project{}, projects.ErrNotFound
			},
		}
		req := newTestRequest("POST", sourceID.String(), body, true)
		rec := httptest.NewRecorder()

		handleCopyExperiment(store, projectStore)(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body=%s)", rec.Code, rec.Body.String())
		}
		if b := decodeEnvelope(t, rec); b.Error == nil || b.Error.Message != "project not found" {
			t.Errorf("error = %+v, want the message to name the project", b.Error)
		}
	})

	// The other half of the pair: a destination the caller owns, but a
	// source that is not theirs. Same 404, different subject.
	t.Run("another user's source experiment is a 404", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			copyFn: func(ctx context.Context, id, userID, targetProjectID uuid.UUID) (experiments.Experiment, error) {
				return experiments.Experiment{}, experiments.ErrNotFound
			},
		}
		req := newTestRequest("POST", uuid.New().String(), body, true)
		rec := httptest.NewRecorder()

		handleCopyExperiment(store, ownedProjectStore(t))(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body=%s)", rec.Code, rec.Body.String())
		}
		if b := decodeEnvelope(t, rec); b.Error == nil || b.Error.Message != "experiment not found" {
			t.Errorf("error = %+v, want the message to name the experiment", b.Error)
		}
	})

	t.Run("store failure", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			copyFn: func(ctx context.Context, id, userID, targetProjectID uuid.UUID) (experiments.Experiment, error) {
				return experiments.Experiment{}, errors.New("db is down")
			},
		}
		req := newTestRequest("POST", sourceID.String(), body, true)
		rec := httptest.NewRecorder()

		handleCopyExperiment(store, ownedProjectStore(t))(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	t.Run("success returns the copy, not the source", func(t *testing.T) {
		title := "落下運動の測定"
		rawData := map[string]any{"columns": map[string]any{"x": []any{1.0}}}
		config := map[string]any{"x_axis_label": "t"}
		var gotID, gotUserID, gotTarget uuid.UUID
		copyID := uuid.New()
		store := &fakeStore{
			t: t,
			copyFn: func(ctx context.Context, id, userID, targetProjectID uuid.UUID) (experiments.Experiment, error) {
				gotID, gotUserID, gotTarget = id, userID, targetProjectID
				// What the real insert returns: a new id, the destination
				// project, and the source's data.
				return experiments.Experiment{
					ID: copyID, UserID: userID, ProjectID: targetProjectID,
					Title: &title, RawData: rawData, Config: config,
				}, nil
			},
		}
		req := newTestRequest("POST", sourceID.String(), body, true)
		rec := httptest.NewRecorder()

		handleCopyExperiment(store, ownedProjectStore(t))(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
		}
		if gotID != sourceID {
			t.Errorf("source id passed to store = %v, want %v", gotID, sourceID)
		}
		if gotUserID != testUserID {
			t.Errorf("userID passed to store = %v, want %v", gotUserID, testUserID)
		}
		if gotTarget != targetProjectID {
			t.Errorf("destination passed to store = %v, want %v", gotTarget, targetProjectID)
		}

		var got experiments.Experiment
		data, err := json.Marshal(decodeEnvelope(t, rec).Data)
		if err != nil {
			t.Fatalf("re-marshal data: %v", err)
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("decode data: %v", err)
		}
		if got.ID == sourceID {
			t.Error("the response carries the source id, want the copy's new id")
		}
		if got.ID != copyID {
			t.Errorf("id = %v, want the copy's %v", got.ID, copyID)
		}
		if got.ProjectID != targetProjectID {
			t.Errorf("project_id = %v, want the destination %v", got.ProjectID, targetProjectID)
		}
		if got.Title == nil || *got.Title != title {
			t.Errorf("title = %v, want %q copied from the source", got.Title, title)
		}
		if !reflect.DeepEqual(got.RawData, rawData) {
			t.Errorf("raw_data = %+v, want the source's %+v", got.RawData, rawData)
		}
		if !reflect.DeepEqual(got.Config, config) {
			t.Errorf("config = %+v, want the source's %+v", got.Config, config)
		}
	})
}

func TestHandleMoveExperiment(t *testing.T) {
	sourceID := uuid.New()
	targetProjectID := uuid.New()
	body := `{"project_id":"` + targetProjectID.String() + `"}`

	ownedProjectStore := func(t *testing.T) *fakeProjectStore {
		return &fakeProjectStore{
			t: t,
			getByIDFn: func(ctx context.Context, id, userID uuid.UUID) (projects.Project, error) {
				return projects.Project{ID: id, UserID: userID}, nil
			},
		}
	}

	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("PATCH", sourceID.String(), body, false)
		rec := httptest.NewRecorder()

		handleMoveExperiment(store, &fakeProjectStore{t: t})(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid experiment id", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("PATCH", "not-a-uuid", body, true)
		rec := httptest.NewRecorder()

		handleMoveExperiment(store, &fakeProjectStore{t: t})(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if b := decodeEnvelope(t, rec); b.Error == nil || b.Error.Code != "invalid_id" {
			t.Errorf("error code = %+v, want invalid_id", b.Error)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("PATCH", sourceID.String(), `not json`, true)
		rec := httptest.NewRecorder()

		handleMoveExperiment(store, &fakeProjectStore{t: t})(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if b := decodeEnvelope(t, rec); b.Error == nil || b.Error.Code != "invalid_body" {
			t.Errorf("error code = %+v, want invalid_body", b.Error)
		}
	})

	// A move with no destination has nothing to fall back on: there is no
	// "default" project to land in, and doing nothing silently would look
	// like the move succeeded.
	t.Run("missing project_id", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("PATCH", sourceID.String(), `{}`, true)
		rec := httptest.NewRecorder()

		handleMoveExperiment(store, &fakeProjectStore{t: t})(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if b := decodeEnvelope(t, rec); b.Error == nil || b.Error.Code != "invalid_project_id" {
			t.Errorf("error code = %+v, want invalid_project_id", b.Error)
		}
	})

	t.Run("malformed project_id", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("PATCH", sourceID.String(), `{"project_id":"not-a-uuid"}`, true)
		rec := httptest.NewRecorder()

		handleMoveExperiment(store, &fakeProjectStore{t: t})(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if b := decodeEnvelope(t, rec); b.Error == nil || b.Error.Code != "invalid_project_id" {
			t.Errorf("error code = %+v, want invalid_project_id", b.Error)
		}
	})

	// Both fakes fail the test if UpdateProject is reached, so this also
	// proves nothing is written when the destination is not the caller's.
	t.Run("another user's destination project is a 404", func(t *testing.T) {
		store := &fakeStore{t: t}
		projectStore := &fakeProjectStore{
			t: t,
			getByIDFn: func(ctx context.Context, id, userID uuid.UUID) (projects.Project, error) {
				return projects.Project{}, projects.ErrNotFound
			},
		}
		req := newTestRequest("PATCH", sourceID.String(), body, true)
		rec := httptest.NewRecorder()

		handleMoveExperiment(store, projectStore)(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body=%s)", rec.Code, rec.Body.String())
		}
		if b := decodeEnvelope(t, rec); b.Error == nil || b.Error.Message != "project not found" {
			t.Errorf("error = %+v, want the message to name the project", b.Error)
		}
	})

	t.Run("another user's experiment is a 404", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			updateProjectFn: func(ctx context.Context, id, userID, targetProjectID uuid.UUID) (experiments.Experiment, error) {
				return experiments.Experiment{}, experiments.ErrNotFound
			},
		}
		req := newTestRequest("PATCH", uuid.New().String(), body, true)
		rec := httptest.NewRecorder()

		handleMoveExperiment(store, ownedProjectStore(t))(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body=%s)", rec.Code, rec.Body.String())
		}
		if b := decodeEnvelope(t, rec); b.Error == nil || b.Error.Message != "experiment not found" {
			t.Errorf("error = %+v, want the message to name the experiment", b.Error)
		}
	})

	t.Run("store failure", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			updateProjectFn: func(ctx context.Context, id, userID, targetProjectID uuid.UUID) (experiments.Experiment, error) {
				return experiments.Experiment{}, errors.New("db is down")
			},
		}
		req := newTestRequest("PATCH", sourceID.String(), body, true)
		rec := httptest.NewRecorder()

		handleMoveExperiment(store, ownedProjectStore(t))(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
	})

	// The id must survive the move -- that is the whole difference from
	// copy-then-delete, which would hand back a different experiment and
	// break every existing link to this one.
	t.Run("success keeps the id and changes the project", func(t *testing.T) {
		var gotID, gotUserID, gotTarget uuid.UUID
		store := &fakeStore{
			t: t,
			updateProjectFn: func(ctx context.Context, id, userID, targetProjectID uuid.UUID) (experiments.Experiment, error) {
				gotID, gotUserID, gotTarget = id, userID, targetProjectID
				return experiments.Experiment{
					ID: id, UserID: userID, ProjectID: targetProjectID,
				}, nil
			},
		}
		req := newTestRequest("PATCH", sourceID.String(), body, true)
		rec := httptest.NewRecorder()

		handleMoveExperiment(store, ownedProjectStore(t))(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		if gotID != sourceID {
			t.Errorf("experiment id passed to store = %v, want %v", gotID, sourceID)
		}
		if gotUserID != testUserID {
			t.Errorf("userID passed to store = %v, want %v", gotUserID, testUserID)
		}
		if gotTarget != targetProjectID {
			t.Errorf("destination passed to store = %v, want %v", gotTarget, targetProjectID)
		}
		if !strings.Contains(rec.Body.String(), sourceID.String()) {
			t.Errorf("body = %s, want it to still carry the original id %v", rec.Body.String(), sourceID)
		}
		if !strings.Contains(rec.Body.String(), targetProjectID.String()) {
			t.Errorf("body = %s, want project_id %v", rec.Body.String(), targetProjectID)
		}
	})

	// Moving into the project it is already in is a no-op, not an error:
	// the row still matches, so the store returns it and this is a plain
	// 200. Pinned so nobody "fixes" it into a 400.
	t.Run("moving to the current project succeeds unchanged", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			updateProjectFn: func(ctx context.Context, id, userID, targetProjectID uuid.UUID) (experiments.Experiment, error) {
				return experiments.Experiment{ID: id, UserID: userID, ProjectID: targetProjectID}, nil
			},
		}
		req := newTestRequest("PATCH", sourceID.String(), body, true)
		rec := httptest.NewRecorder()

		handleMoveExperiment(store, ownedProjectStore(t))(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleGetExperiment(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("GET", uuid.New().String(), "", false)
		rec := httptest.NewRecorder()

		handleGetExperiment(store)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("GET", "not-a-uuid", "", true)
		rec := httptest.NewRecorder()

		handleGetExperiment(store)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_id" {
			t.Errorf("error code = %+v, want invalid_id", body.Error)
		}
	})

	t.Run("not found", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			getByIDFn: func(ctx context.Context, id, userID uuid.UUID) (experiments.Experiment, error) {
				return experiments.Experiment{}, experiments.ErrNotFound
			},
		}
		req := newTestRequest("GET", uuid.New().String(), "", true)
		rec := httptest.NewRecorder()

		handleGetExperiment(store)(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	// Every other fake here returns the bare sentinel, which the mapping would
	// recognize even compared with ==. This one adds context the way a real
	// store reasonably might, so a regression from errors.Is back to == shows
	// up as a 500 here instead of hiding until production (KAN-66).
	t.Run("not found, wrapped by the store", func(t *testing.T) {
		id := uuid.New()
		store := &fakeStore{
			t: t,
			getByIDFn: func(ctx context.Context, gotID, userID uuid.UUID) (experiments.Experiment, error) {
				return experiments.Experiment{}, fmt.Errorf("get experiment %s: %w", gotID, experiments.ErrNotFound)
			},
		}
		req := newTestRequest("GET", id.String(), "", true)
		rec := httptest.NewRecorder()

		handleGetExperiment(store)(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "not_found" {
			t.Errorf("error = %+v, want not_found", body.Error)
		}
	})

	t.Run("success", func(t *testing.T) {
		id := uuid.New()
		store := &fakeStore{
			t: t,
			getByIDFn: func(ctx context.Context, gotID, userID uuid.UUID) (experiments.Experiment, error) {
				if gotID != id {
					t.Errorf("id = %v, want %v", gotID, id)
				}
				if userID != testUserID {
					t.Errorf("userID = %v, want %v", userID, testUserID)
				}
				return experiments.Experiment{ID: id, UserID: userID}, nil
			},
		}
		req := newTestRequest("GET", id.String(), "", true)
		rec := httptest.NewRecorder()

		handleGetExperiment(store)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleListExperiments(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("GET", "", "", false)
		rec := httptest.NewRecorder()

		handleListExperiments(store)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			listByUserFn: func(ctx context.Context, userID uuid.UUID) ([]experiments.Experiment, error) {
				return []experiments.Experiment{}, nil
			},
		}
		req := newTestRequest("GET", "", "", true)
		rec := httptest.NewRecorder()

		handleListExperiments(store)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		body := decodeEnvelope(t, rec)
		list, ok := body.Data.([]any)
		if !ok || len(list) != 0 {
			t.Errorf("data = %+v, want an empty array", body.Data)
		}
	})

	t.Run("success returns the user's experiments", func(t *testing.T) {
		var gotUserID uuid.UUID
		store := &fakeStore{
			t: t,
			listByUserFn: func(ctx context.Context, userID uuid.UUID) ([]experiments.Experiment, error) {
				gotUserID = userID
				return []experiments.Experiment{
					{ID: uuid.New(), UserID: userID},
					{ID: uuid.New(), UserID: userID},
				}, nil
			},
		}
		req := newTestRequest("GET", "", "", true)
		rec := httptest.NewRecorder()

		handleListExperiments(store)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		if gotUserID != testUserID {
			t.Errorf("userID passed to store = %v, want %v", gotUserID, testUserID)
		}
		body := decodeEnvelope(t, rec)
		list, ok := body.Data.([]any)
		if !ok || len(list) != 2 {
			t.Errorf("data = %+v, want 2 experiments", body.Data)
		}
	})
}

func TestHandleDeleteExperiment(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("DELETE", uuid.New().String(), "", false)
		rec := httptest.NewRecorder()

		handleDeleteExperiment(store)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("DELETE", "not-a-uuid", "", true)
		rec := httptest.NewRecorder()

		handleDeleteExperiment(store)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_id" {
			t.Errorf("error code = %+v, want invalid_id", body.Error)
		}
	})

	t.Run("not found", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			deleteFn: func(ctx context.Context, id, userID uuid.UUID) error {
				return experiments.ErrNotFound
			},
		}
		req := newTestRequest("DELETE", uuid.New().String(), "", true)
		rec := httptest.NewRecorder()

		handleDeleteExperiment(store)(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		id := uuid.New()
		var gotID, gotUserID uuid.UUID
		store := &fakeStore{
			t: t,
			deleteFn: func(ctx context.Context, id, userID uuid.UUID) error {
				gotID = id
				gotUserID = userID
				return nil
			},
		}
		req := newTestRequest("DELETE", id.String(), "", true)
		rec := httptest.NewRecorder()

		handleDeleteExperiment(store)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		if gotID != id {
			t.Errorf("id passed to store = %v, want %v", gotID, id)
		}
		if gotUserID != testUserID {
			t.Errorf("userID passed to store = %v, want %v", gotUserID, testUserID)
		}
	})
}

func TestHandleUpdateExperimentConfig(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("PATCH", uuid.New().String(), `{"config":{}}`, false)
		rec := httptest.NewRecorder()

		handleUpdateExperimentConfig(store)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("PATCH", "not-a-uuid", `{"config":{}}`, true)
		rec := httptest.NewRecorder()

		handleUpdateExperimentConfig(store)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_id" {
			t.Errorf("error code = %+v, want invalid_id", body.Error)
		}
	})

	t.Run("missing config", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("PATCH", uuid.New().String(), `{}`, true)
		rec := httptest.NewRecorder()

		handleUpdateExperimentConfig(store)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_config" {
			t.Errorf("error code = %+v, want invalid_config", body.Error)
		}
	})

	t.Run("not found", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			updateConfigFn: func(ctx context.Context, id, userID uuid.UUID, config map[string]any) (experiments.Experiment, error) {
				return experiments.Experiment{}, experiments.ErrNotFound
			},
		}
		req := newTestRequest("PATCH", uuid.New().String(), `{"config":{"x_axis_label":"v"}}`, true)
		rec := httptest.NewRecorder()

		handleUpdateExperimentConfig(store)(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		id := uuid.New()
		store := &fakeStore{
			t: t,
			updateConfigFn: func(ctx context.Context, gotID, userID uuid.UUID, config map[string]any) (experiments.Experiment, error) {
				return experiments.Experiment{ID: gotID, UserID: userID, Config: config}, nil
			},
		}
		req := newTestRequest("PATCH", id.String(), `{"config":{"x_axis_label":"v"}}`, true)
		rec := httptest.NewRecorder()

		handleUpdateExperimentConfig(store)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleUpdateExperimentRawData(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("PATCH", uuid.New().String(), `{"raw_data":{}}`, false)
		rec := httptest.NewRecorder()

		handleUpdateExperimentRawData(store, newFakeCache())(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("PATCH", "not-a-uuid", `{"raw_data":{}}`, true)
		rec := httptest.NewRecorder()

		handleUpdateExperimentRawData(store, newFakeCache())(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_id" {
			t.Errorf("error code = %+v, want invalid_id", body.Error)
		}
	})

	t.Run("missing raw_data", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("PATCH", uuid.New().String(), `{}`, true)
		rec := httptest.NewRecorder()

		handleUpdateExperimentRawData(store, newFakeCache())(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_raw_data" {
			t.Errorf("error code = %+v, want invalid_raw_data", body.Error)
		}
	})

	t.Run("not found", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			updateRawDataFn: func(ctx context.Context, id, userID uuid.UUID, rawData map[string]any) (experiments.Experiment, error) {
				return experiments.Experiment{}, experiments.ErrNotFound
			},
		}
		req := newTestRequest("PATCH", uuid.New().String(), `{"raw_data":{"columns":{"x":[1]}}}`, true)
		rec := httptest.NewRecorder()

		handleUpdateExperimentRawData(store, newFakeCache())(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("success invalidates any cached analysis results for the experiment", func(t *testing.T) {
		id := uuid.New()
		fc := newFakeCache()
		staleKey, err := cache.AnalysisKey(id, "linear_regression", map[string]any{})
		if err != nil {
			t.Fatalf("compute cache key: %v", err)
		}
		fc.store[staleKey] = []byte(`{"data":{"result":{"slope":1}},"error":null,"meta":{}}`)
		otherKey, err := cache.AnalysisKey(uuid.New(), "linear_regression", map[string]any{})
		if err != nil {
			t.Fatalf("compute cache key: %v", err)
		}
		fc.store[otherKey] = []byte(`{"data":{"result":{"slope":9}},"error":null,"meta":{}}`)

		var gotID, gotUserID uuid.UUID
		var gotRawData map[string]any
		store := &fakeStore{
			t: t,
			updateRawDataFn: func(ctx context.Context, id, userID uuid.UUID, rawData map[string]any) (experiments.Experiment, error) {
				gotID = id
				gotUserID = userID
				gotRawData = rawData
				return experiments.Experiment{ID: id, UserID: userID, RawData: rawData}, nil
			},
		}
		req := newTestRequest("PATCH", id.String(), `{"raw_data":{"columns":{"x":[1,2],"y":[2,4]}}}`, true)
		rec := httptest.NewRecorder()

		handleUpdateExperimentRawData(store, fc)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		if gotID != id {
			t.Errorf("id passed to store = %v, want %v", gotID, id)
		}
		if gotUserID != testUserID {
			t.Errorf("userID passed to store = %v, want %v", gotUserID, testUserID)
		}
		if gotRawData == nil {
			t.Fatal("raw_data was not passed to store")
		}
		if _, ok := fc.store[staleKey]; ok {
			t.Error("stale cached analysis result for this experiment was not invalidated")
		}
		if _, ok := fc.store[otherKey]; !ok {
			t.Error("another experiment's cached analysis result was incorrectly invalidated")
		}
	})
}

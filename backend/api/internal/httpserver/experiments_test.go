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

	"github.com/google/uuid"

	"analyseapp/api/internal/cache"
	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/projects"
	"analyseapp/api/internal/response"
)

// experimentStores is the pair of fakes the handlers that touch both
// experiments and projects take. Every method of both fails the test unless
// a row sets it.
type experimentStores struct {
	exp  *fakeStore
	proj *fakeProjectStore
}

func newExperimentStores(t *testing.T) experimentStores {
	return experimentStores{exp: &fakeStore{t: t}, proj: &fakeProjectStore{t: t}}
}

// wantCreated is what a create handler must hand Store.Create, all of it
// taken from the request (or, for the flat path, the default project) and
// none of it invented.
type wantCreated struct {
	projectID       uuid.UUID
	title           *string
	rawData, config map[string]any
}

// creating returns a Create that checks it was handed want on behalf of the
// authenticated user, then answers err or the experiment built from it.
func creating(t *testing.T, want wantCreated, err error) func(context.Context, uuid.UUID, uuid.UUID, *string, map[string]any, map[string]any) (experiments.Experiment, error) {
	return func(_ context.Context, userID, projectID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error) {
		if userID != testUserID {
			t.Errorf("userID passed to store = %v, want the authenticated user %v", userID, testUserID)
		}
		if projectID != want.projectID {
			t.Errorf("project id passed to store = %v, want %v", projectID, want.projectID)
		}
		if !reflect.DeepEqual(title, want.title) {
			t.Errorf("title passed to store = %v, want %v", deref(title), deref(want.title))
		}
		if !reflect.DeepEqual(rawData, want.rawData) || !reflect.DeepEqual(config, want.config) {
			t.Errorf("raw_data, config passed to store = %#v, %#v, want %#v, %#v", rawData, config, want.rawData, want.config)
		}
		if err != nil {
			return experiments.Experiment{}, err
		}
		return experiments.Experiment{ID: uuid.New(), UserID: userID, ProjectID: projectID, Title: title, RawData: rawData, Config: config}, nil
	}
}

func deref(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// wantData checks the response carries field = want.
func wantData(field string, want any) func(t *testing.T, body response.Envelope) {
	return func(t *testing.T, body response.Envelope) {
		data, _ := body.Data.(map[string]any)
		if data[field] != want {
			t.Errorf("data.%s = %v, want %v (data=%+v)", field, data[field], want, body.Data)
		}
	}
}

// wantMessage checks the error's message, where the code alone does not
// say enough (which resource a 404 names).
func wantMessage(want string) func(t *testing.T, body response.Envelope) {
	return func(t *testing.T, body response.Envelope) {
		if body.Error == nil || body.Error.Message != want {
			t.Errorf("error = %+v, want message %q", body.Error, want)
		}
	}
}

func TestHandleCreateExperiment(t *testing.T) {
	defaultProject := projects.Project{ID: uuid.New(), Title: projects.DefaultTitle}
	rawData := map[string]any{"columns": map[string]any{"x": []any{1.0}}}
	config := map[string]any{"x_axis_label": "t"}
	const body = `{"title":"my experiment","raw_data":{"columns":{"x":[1]}},"config":{"x_axis_label":"t"}}`

	// stores builds the success path's fakes: a profile ensured, the default
	// project resolved, and a Create that checks what it was handed -- and
	// that it came after the profile, whose row experiments.user_id
	// references.
	stores := func(want wantCreated, createErr error) func(t *testing.T) experimentStores {
		return func(t *testing.T) experimentStores {
			ensured := false
			create := creating(t, want, createErr)
			return experimentStores{
				exp: &fakeStore{
					t: t,
					ensureProfileFn: func(_ context.Context, userID uuid.UUID) error {
						ensured = true
						return nil
					},
					createFn: func(ctx context.Context, userID, projectID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error) {
						if !ensured {
							t.Error("Create was called before EnsureProfile")
						}
						return create(ctx, userID, projectID, title, rawData, config)
					},
				},
				proj: defaultProjectStore(t, defaultProject),
			}
		}
	}
	want := wantCreated{projectID: defaultProject.ID, title: new("my experiment"), rawData: rawData, config: config}

	runHandlerCases(t, "POST", []handlerCase[experimentStores]{
		{name: "unauthenticated", body: body, unauthenticated: true,
			wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "invalid JSON body", body: `not json`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_body"},
		{name: "missing raw_data", body: `{"title":"x"}`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_raw_data"},
		// The flat path names no project, so the handler has to supply one:
		// without it the insert violates experiments.project_id's NOT NULL.
		{name: "success stores the experiment in the user's default project", body: body,
			store: stores(want, nil), wantStatus: http.StatusCreated,
			check: wantData("project_id", defaultProject.ID.String())},
		{name: "empty title is normalized to nil", body: `{"title":"","raw_data":{"columns":{"x":[1]}}}`,
			store:      stores(wantCreated{projectID: defaultProject.ID, rawData: rawData}, nil),
			wantStatus: http.StatusCreated},
		{name: "profile cannot be ensured",
			body: body,
			store: func(t *testing.T) experimentStores {
				return experimentStores{
					exp: &fakeStore{t: t, ensureProfileFn: func(context.Context, uuid.UUID) error {
						return errStoreDown
					}},
					proj: &fakeProjectStore{t: t},
				}
			},
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
		// Nothing can be stored without a project, so this is a 500 rather
		// than an experiment quietly created somewhere else.
		{name: "default project cannot be resolved", body: body,
			store: func(t *testing.T) experimentStores {
				return experimentStores{
					exp: &fakeStore{t: t},
					proj: &fakeProjectStore{t: t, ensureDefaultFn: func(context.Context, uuid.UUID) (projects.Project, error) {
						return projects.Project{}, errStoreDown
					}},
				}
			},
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
		{name: "store failure", body: body, store: stores(want, errStoreDown),
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}, newExperimentStores, func(s experimentStores) http.HandlerFunc {
		return handleCreateExperiment(s.exp, s.proj)
	})
}

func TestHandleCreateProjectExperiment(t *testing.T) {
	projectID := uuid.New()
	rawData := map[string]any{"columns": map[string]any{"x": []any{1.0}}}
	const body = `{"title":"落下運動","raw_data":{"columns":{"x":[1]}}}`

	// No EnsureProfile here, unlike the flat path (see the handler): the
	// fake fails the test if it is called.
	stores := func(want wantCreated, err error) func(t *testing.T) *fakeStore {
		return func(t *testing.T) *fakeStore {
			return &fakeStore{
				t:        t,
				createFn: creating(t, want, err),
				ensureProfileFn: func(context.Context, uuid.UUID) error {
					t.Error("unexpected call to EnsureProfile")
					return nil
				},
			}
		}
	}
	want := wantCreated{projectID: projectID, title: new("落下運動"), rawData: rawData}

	runHandlerCases(t, "POST", []handlerCase[*fakeStore]{
		{name: "unauthenticated", id: projectID.String(), body: body, unauthenticated: true,
			wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "invalid project id", id: "not-a-uuid", body: body,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "invalid JSON body", id: projectID.String(), body: `not json`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_body"},
		{name: "missing raw_data", id: projectID.String(), body: `{"title":"x"}`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_raw_data"},
		{name: "success stores the experiment in the project from the URL", id: projectID.String(), body: body,
			store: stores(want, nil), wantStatus: http.StatusCreated,
			check: wantData("project_id", projectID.String())},
		{name: "empty title is normalized to nil", id: projectID.String(),
			body:       `{"title":"","raw_data":{"columns":{"x":[1]}}}`,
			store:      stores(wantCreated{projectID: projectID, rawData: rawData}, nil),
			wantStatus: http.StatusCreated},
		// Another user's project id, and an id that names nothing, are the
		// same case: the store's insert is guarded by ownership, so both
		// write nothing and come back as ErrNotFound. What matters is that
		// the reply is the 404 GET /api/v1/projects/{id} gives -- otherwise
		// this endpoint would confirm which project ids exist.
		{name: "another user's project is a 404 naming the project", id: projectID.String(), body: body,
			store:      stores(want, experiments.ErrNotFound),
			wantStatus: http.StatusNotFound, wantCode: "not_found", check: wantMessage("project not found")},
		{name: "store failure", id: projectID.String(), body: body,
			store:      stores(want, errStoreDown),
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}, newExperimentStore, func(s *fakeStore) http.HandlerFunc { return handleCreateProjectExperiment(s) })
}

func TestHandleListProjectExperiments(t *testing.T) {
	projectID := uuid.New()

	// project answers GetByID as the project store would: with the project
	// for its owner, or err. The list handler reads nothing else off it.
	project := func(t *testing.T, err error) *fakeProjectStore {
		return &fakeProjectStore{t: t, getByIDFn: func(_ context.Context, id, userID uuid.UUID) (projects.Project, error) {
			wantStoreCall(t, id, projectID, userID)
			if err != nil {
				return projects.Project{}, err
			}
			return projects.Project{ID: id, UserID: userID}, nil
		}}
	}
	// listing answers ListByProject with n experiments, or err.
	listing := func(t *testing.T, n int, err error) *fakeStore {
		return &fakeStore{t: t, listByProjectFn: func(_ context.Context, gotProjectID, userID uuid.UUID) ([]experiments.Experiment, error) {
			wantStoreCall(t, gotProjectID, projectID, userID)
			if err != nil {
				return nil, err
			}
			list := []experiments.Experiment{}
			for range n {
				list = append(list, experiments.Experiment{ID: uuid.New(), UserID: userID, ProjectID: gotProjectID})
			}
			return list, nil
		}}
	}
	wantLen := func(n int) func(t *testing.T, body response.Envelope) {
		return func(t *testing.T, body response.Envelope) {
			if list, ok := body.Data.([]any); !ok || len(list) != n {
				t.Errorf("data = %+v, want an array of %d", body.Data, n)
			}
		}
	}

	runHandlerCases(t, "GET", []handlerCase[experimentStores]{
		{name: "unauthenticated", id: projectID.String(), unauthenticated: true,
			wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "invalid project id", id: "not-a-uuid",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "success scopes the query to the project and the caller", id: projectID.String(),
			store: func(t *testing.T) experimentStores {
				return experimentStores{exp: listing(t, 2, nil), proj: project(t, nil)}
			},
			wantStatus: http.StatusOK, check: wantLen(2)},
		// A project of the user's own that holds nothing answers 200 with an
		// empty array -- the case the 404 below must stay distinguishable
		// from.
		{name: "an empty project is an empty array, not a 404", id: projectID.String(),
			store: func(t *testing.T) experimentStores {
				return experimentStores{exp: listing(t, 0, nil), proj: project(t, nil)}
			},
			wantStatus: http.StatusOK, check: wantLen(0)},
		// The whole reason the handler reads the project first. The
		// experiments fake fails the test if ListByProject is reached, so a
		// 404 here also proves no query ran against someone else's project.
		{name: "another user's project is a 404", id: projectID.String(),
			store: func(t *testing.T) experimentStores {
				return experimentStores{exp: &fakeStore{t: t}, proj: project(t, projects.ErrNotFound)}
			},
			wantStatus: http.StatusNotFound, wantCode: "not_found", check: wantMessage("project not found")},
		{name: "project lookup failure", id: projectID.String(),
			store: func(t *testing.T) experimentStores {
				return experimentStores{exp: &fakeStore{t: t}, proj: project(t, errStoreDown)}
			},
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
		{name: "list failure", id: projectID.String(),
			store: func(t *testing.T) experimentStores {
				return experimentStores{exp: listing(t, 0, errStoreDown), proj: project(t, nil)}
			},
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}, newExperimentStores, func(s experimentStores) http.HandlerFunc {
		return handleListProjectExperiments(s.exp, s.proj)
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

// newExperimentStore is the default store for the tables below: every
// method fails the test if called.
func newExperimentStore(t *testing.T) *fakeStore { return &fakeStore{t: t} }

func TestHandleGetExperiment(t *testing.T) {
	id := uuid.New()
	getByID := func(fn func(id, userID uuid.UUID) (experiments.Experiment, error)) func(t *testing.T) *fakeStore {
		return func(t *testing.T) *fakeStore {
			return &fakeStore{t: t, getByIDFn: func(_ context.Context, id, userID uuid.UUID) (experiments.Experiment, error) {
				return fn(id, userID)
			}}
		}
	}

	runHandlerCases(t, "GET", []handlerCase[*fakeStore]{
		{name: "unauthenticated", id: id.String(), unauthenticated: true,
			wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "invalid id", id: "not-a-uuid",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "success returns the stored experiment", id: id.String(),
			store: func(t *testing.T) *fakeStore {
				return getByID(func(gotID, userID uuid.UUID) (experiments.Experiment, error) {
					wantStoreCall(t, gotID, id, userID)
					return experiments.Experiment{ID: gotID, UserID: userID, Title: new("run 1")}, nil
				})(t)
			},
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body response.Envelope) {
				data, _ := body.Data.(map[string]any)
				if data["id"] != id.String() || data["title"] != "run 1" {
					t.Errorf("data = %+v, want the stored experiment", body.Data)
				}
			}},
		{name: "not found", id: id.String(),
			store: getByID(func(uuid.UUID, uuid.UUID) (experiments.Experiment, error) {
				return experiments.Experiment{}, experiments.ErrNotFound
			}),
			wantStatus: http.StatusNotFound, wantCode: "not_found"},
		// Every other fake returns the bare sentinel, which the mapping would
		// recognize even compared with ==. This one adds context the way a
		// real store reasonably might, so a regression from errors.Is back to
		// == shows up as a 500 here instead of hiding until production
		// (KAN-66).
		{name: "not found, wrapped by the store", id: id.String(),
			store: getByID(func(gotID, _ uuid.UUID) (experiments.Experiment, error) {
				return experiments.Experiment{}, fmt.Errorf("get experiment %s: %w", gotID, experiments.ErrNotFound)
			}),
			wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "store failure", id: id.String(),
			store: getByID(func(uuid.UUID, uuid.UUID) (experiments.Experiment, error) {
				return experiments.Experiment{}, errStoreDown
			}),
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}, newExperimentStore, func(s *fakeStore) http.HandlerFunc { return handleGetExperiment(s) })
}

func TestHandleListExperiments(t *testing.T) {
	listByUser := func(n int) func(t *testing.T) *fakeStore {
		return func(t *testing.T) *fakeStore {
			return &fakeStore{t: t, listByUserFn: func(_ context.Context, userID uuid.UUID) ([]experiments.Experiment, error) {
				if userID != testUserID {
					t.Errorf("userID passed to store = %v, want the authenticated user %v", userID, testUserID)
				}
				list := []experiments.Experiment{}
				for range n {
					list = append(list, experiments.Experiment{ID: uuid.New(), UserID: userID})
				}
				return list, nil
			}}
		}
	}
	wantLen := func(n int) func(t *testing.T, body response.Envelope) {
		return func(t *testing.T, body response.Envelope) {
			if list, ok := body.Data.([]any); !ok || len(list) != n {
				t.Errorf("data = %+v, want an array of %d", body.Data, n)
			}
		}
	}

	runHandlerCases(t, "GET", []handlerCase[*fakeStore]{
		{name: "unauthenticated", unauthenticated: true,
			wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "success returns the user's experiments", store: listByUser(2),
			wantStatus: http.StatusOK, check: wantLen(2)},
		// An empty array, not null: the frontend calls .map() on this.
		{name: "no experiments is an empty array", store: listByUser(0),
			wantStatus: http.StatusOK, check: wantLen(0)},
		{name: "store failure",
			store: func(t *testing.T) *fakeStore {
				return &fakeStore{t: t, listByUserFn: func(context.Context, uuid.UUID) ([]experiments.Experiment, error) {
					return nil, errStoreDown
				}}
			},
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}, newExperimentStore, func(s *fakeStore) http.HandlerFunc { return handleListExperiments(s) })
}

func TestHandleDeleteExperiment(t *testing.T) {
	id := uuid.New()
	deleteReturning := func(err error) func(t *testing.T) *fakeStore {
		return func(t *testing.T) *fakeStore {
			return &fakeStore{t: t, deleteFn: func(_ context.Context, gotID, userID uuid.UUID) error {
				wantStoreCall(t, gotID, id, userID)
				return err
			}}
		}
	}

	runHandlerCases(t, "DELETE", []handlerCase[*fakeStore]{
		{name: "unauthenticated", id: id.String(), unauthenticated: true,
			wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "invalid id", id: "not-a-uuid",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "success echoes the deleted id", id: id.String(), store: deleteReturning(nil),
			wantStatus: http.StatusOK,
			check: func(t *testing.T, body response.Envelope) {
				if data, _ := body.Data.(map[string]any); data["id"] != id.String() {
					t.Errorf("data = %+v, want {id: %s}", body.Data, id)
				}
			}},
		{name: "not found", id: id.String(), store: deleteReturning(experiments.ErrNotFound),
			wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "store failure", id: id.String(), store: deleteReturning(errStoreDown),
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}, newExperimentStore, func(s *fakeStore) http.HandlerFunc { return handleDeleteExperiment(s) })
}

func TestHandleUpdateExperimentConfig(t *testing.T) {
	id := uuid.New()
	// updateConfig checks the store is handed the request's config as-is,
	// then answers with err, or else with the experiment carrying it.
	updateConfig := func(wantConfig map[string]any, err error) func(t *testing.T) *fakeStore {
		return func(t *testing.T) *fakeStore {
			return &fakeStore{t: t, updateConfigFn: func(_ context.Context, gotID, userID uuid.UUID, config map[string]any) (experiments.Experiment, error) {
				wantStoreCall(t, gotID, id, userID)
				if !reflect.DeepEqual(config, wantConfig) {
					t.Errorf("config passed to store = %#v, want %#v", config, wantConfig)
				}
				if err != nil {
					return experiments.Experiment{}, err
				}
				return experiments.Experiment{ID: gotID, UserID: userID, Config: config}, nil
			}}
		}
	}
	wantDataConfig := func(want map[string]any) func(t *testing.T, body response.Envelope) {
		return func(t *testing.T, body response.Envelope) {
			data, _ := body.Data.(map[string]any)
			if !reflect.DeepEqual(data["config"], want) {
				t.Errorf("data.config = %#v, want %#v", data["config"], want)
			}
		}
	}
	labels := map[string]any{"x_axis_label": "v", "fit": map[string]any{"formula": "a*x"}}

	runHandlerCases(t, "PATCH", []handlerCase[*fakeStore]{
		{name: "unauthenticated", id: id.String(), body: `{"config":{}}`, unauthenticated: true,
			wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "invalid id", id: "not-a-uuid", body: `{"config":{}}`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "missing config", id: id.String(), body: `{}`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_config"},
		{name: "null config is missing too", id: id.String(), body: `{"config":null}`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_config"},
		{name: "success stores the config as sent", id: id.String(),
			body:       `{"config":{"x_axis_label":"v","fit":{"formula":"a*x"}}}`,
			store:      updateConfig(labels, nil),
			wantStatus: http.StatusOK, check: wantDataConfig(labels)},
		// {} is a config, not a missing one: it clears every setting. The
		// replace-not-merge semantics are why the frontend merges before
		// writing (KAN-29).
		{name: "empty config clears it", id: id.String(), body: `{"config":{}}`,
			store:      updateConfig(map[string]any{}, nil),
			wantStatus: http.StatusOK, check: wantDataConfig(map[string]any{})},
		{name: "not found", id: id.String(), body: `{"config":{"x_axis_label":"v"}}`,
			store:      updateConfig(map[string]any{"x_axis_label": "v"}, experiments.ErrNotFound),
			wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "store failure", id: id.String(), body: `{"config":{"x_axis_label":"v"}}`,
			store:      updateConfig(map[string]any{"x_axis_label": "v"}, errStoreDown),
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}, newExperimentStore, func(s *fakeStore) http.HandlerFunc { return handleUpdateExperimentConfig(s) })
}

func TestHandleUpdateExperimentRawData(t *testing.T) {
	id := uuid.New()
	thisKey, err := cache.AnalysisKey(id, "linear_regression", map[string]any{})
	if err != nil {
		t.Fatalf("compute cache key: %v", err)
	}
	otherKey, err := cache.AnalysisKey(uuid.New(), "linear_regression", map[string]any{})
	if err != nil {
		t.Fatalf("compute cache key: %v", err)
	}

	// Each row gets a fresh cache holding a result for this experiment and
	// one for another, built when the runner builds the row's handler; the
	// row's check reads it afterwards. Rows run in order, never in parallel.
	var fc *fakeCache
	handler := func(s *fakeStore) http.HandlerFunc {
		fc = newFakeCache()
		fc.store[thisKey] = []byte(`{"data":{"result":{"slope":1}},"error":null,"meta":{}}`)
		fc.store[otherKey] = []byte(`{"data":{"result":{"slope":9}},"error":null,"meta":{}}`)
		return handleUpdateExperimentRawData(s, fc)
	}
	updateRawData := func(err error) func(t *testing.T) *fakeStore {
		return func(t *testing.T) *fakeStore {
			return &fakeStore{t: t, updateRawDataFn: func(_ context.Context, gotID, userID uuid.UUID, rawData map[string]any) (experiments.Experiment, error) {
				wantStoreCall(t, gotID, id, userID)
				if rawData == nil {
					t.Error("raw_data was not passed to store")
				}
				if err != nil {
					return experiments.Experiment{}, err
				}
				return experiments.Experiment{ID: gotID, UserID: userID, RawData: rawData}, nil
			}}
		}
	}
	// A failed update left the data as it was, so the cached results still
	// describe it; throwing them away would only cost a recompute. A
	// successful one must drop this experiment's results -- they were
	// computed from the old data and would be served for up to a day --
	// and only this experiment's.
	wantCached := func(this bool) func(t *testing.T, _ response.Envelope) {
		return func(t *testing.T, _ response.Envelope) {
			if _, ok := fc.store[thisKey]; ok != this {
				t.Errorf("this experiment's cached result present = %v, want %v", ok, this)
			}
			if _, ok := fc.store[otherKey]; !ok {
				t.Error("another experiment's cached result was invalidated")
			}
		}
	}
	const body = `{"raw_data":{"columns":{"x":[1,2],"y":[2,4]}}}`

	runHandlerCases(t, "PATCH", []handlerCase[*fakeStore]{
		{name: "unauthenticated", id: id.String(), body: body, unauthenticated: true,
			wantStatus: http.StatusUnauthorized, wantCode: "unauthorized", check: wantCached(true)},
		{name: "invalid id", id: "not-a-uuid", body: body,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_id", check: wantCached(true)},
		{name: "missing raw_data", id: id.String(), body: `{}`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_raw_data", check: wantCached(true)},
		{name: "success invalidates this experiment's cached results", id: id.String(), body: body,
			store: updateRawData(nil), wantStatus: http.StatusOK, check: wantCached(false)},
		{name: "not found leaves the cache alone", id: id.String(), body: body,
			store:      updateRawData(experiments.ErrNotFound),
			wantStatus: http.StatusNotFound, wantCode: "not_found", check: wantCached(true)},
		{name: "store failure leaves the cache alone", id: id.String(), body: body,
			store:      updateRawData(errStoreDown),
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error", check: wantCached(true)},
	}, newExperimentStore, handler)
}

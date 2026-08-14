package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"analyseapp/api/internal/auth"
	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/response"
)

var testUserID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

// fakeStore is a minimal experiments.Store implementation for handler
// tests, so handler validation/status-code logic is testable without a
// database. Unset function fields fail the test if called.
type fakeStore struct {
	t              *testing.T
	createFn       func(ctx context.Context, userID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error)
	getByIDFn      func(ctx context.Context, id, userID uuid.UUID) (experiments.Experiment, error)
	updateConfigFn func(ctx context.Context, id, userID uuid.UUID, config map[string]any) (experiments.Experiment, error)
}

func (f *fakeStore) EnsureProfile(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (f *fakeStore) Create(ctx context.Context, userID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error) {
	if f.createFn == nil {
		f.t.Fatal("unexpected call to Create")
	}
	return f.createFn(ctx, userID, title, rawData, config)
}

func (f *fakeStore) GetByID(ctx context.Context, id, userID uuid.UUID) (experiments.Experiment, error) {
	if f.getByIDFn == nil {
		f.t.Fatal("unexpected call to GetByID")
	}
	return f.getByIDFn(ctx, id, userID)
}

func (f *fakeStore) UpdateConfig(ctx context.Context, id, userID uuid.UUID, config map[string]any) (experiments.Experiment, error) {
	if f.updateConfigFn == nil {
		f.t.Fatal("unexpected call to UpdateConfig")
	}
	return f.updateConfigFn(ctx, id, userID, config)
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

		handleCreateExperiment(store)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		store := &fakeStore{t: t}
		req := newTestRequest("POST", "", `not json`, true)
		rec := httptest.NewRecorder()

		handleCreateExperiment(store)(rec, req)

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

		handleCreateExperiment(store)(rec, req)

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
			createFn: func(ctx context.Context, userID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error) {
				gotTitle = title
				gotTitleSet = true
				return experiments.Experiment{ID: uuid.New(), UserID: userID, Title: title, RawData: rawData, Config: config}, nil
			},
		}
		req := newTestRequest("POST", "", `{"title":"","raw_data":{"columns":{}}}`, true)
		rec := httptest.NewRecorder()

		handleCreateExperiment(store)(rec, req)

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

	t.Run("success", func(t *testing.T) {
		title := "my experiment"
		store := &fakeStore{
			t: t,
			createFn: func(ctx context.Context, userID uuid.UUID, title *string, rawData, config map[string]any) (experiments.Experiment, error) {
				return experiments.Experiment{ID: uuid.New(), UserID: userID, Title: title, RawData: rawData, Config: config}, nil
			},
		}
		req := newTestRequest("POST", "", `{"title":"`+title+`","raw_data":{"columns":{"x":[1]}}}`, true)
		rec := httptest.NewRecorder()

		handleCreateExperiment(store)(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
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

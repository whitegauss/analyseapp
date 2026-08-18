package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"analyseapp/api/internal/projects"
)

// fakeProjectStore is a minimal projects.Store implementation for handler
// tests, so handler validation/status-code logic is testable without a
// database. Unset function fields fail the test if called.
type fakeProjectStore struct {
	t            *testing.T
	createFn     func(ctx context.Context, userID uuid.UUID, title, description string) (projects.Project, error)
	getByIDFn    func(ctx context.Context, id, userID uuid.UUID) (projects.Project, error)
	listByUserFn func(ctx context.Context, userID uuid.UUID) ([]projects.Project, error)
	updateFn     func(ctx context.Context, id, userID uuid.UUID, title, description string) (projects.Project, error)
	deleteFn     func(ctx context.Context, id, userID uuid.UUID) error
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

func TestHandleCreateProject(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeProjectStore{t: t}
		req := newTestRequest("POST", "", `{"title":"x"}`, false)
		rec := httptest.NewRecorder()

		handleCreateProject(store)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid JSON body", func(t *testing.T) {
		store := &fakeProjectStore{t: t}
		req := newTestRequest("POST", "", `not json`, true)
		rec := httptest.NewRecorder()

		handleCreateProject(store)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_body" {
			t.Errorf("error code = %+v, want invalid_body", body.Error)
		}
	})

	t.Run("missing title", func(t *testing.T) {
		store := &fakeProjectStore{t: t}
		req := newTestRequest("POST", "", `{"description":"x"}`, true)
		rec := httptest.NewRecorder()

		handleCreateProject(store)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_title" {
			t.Errorf("error code = %+v, want invalid_title", body.Error)
		}
	})

	t.Run("success", func(t *testing.T) {
		store := &fakeProjectStore{
			t: t,
			createFn: func(ctx context.Context, userID uuid.UUID, title, description string) (projects.Project, error) {
				return projects.Project{ID: uuid.New(), UserID: userID, Title: title, Description: description}, nil
			},
		}
		req := newTestRequest("POST", "", `{"title":"my project","description":"desc"}`, true)
		rec := httptest.NewRecorder()

		handleCreateProject(store)(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("status = %d, want 201 (body=%s)", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleListProjects(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeProjectStore{t: t}
		req := newTestRequest("GET", "", "", false)
		rec := httptest.NewRecorder()

		handleListProjects(store)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		store := &fakeProjectStore{
			t: t,
			listByUserFn: func(ctx context.Context, userID uuid.UUID) ([]projects.Project, error) {
				return []projects.Project{}, nil
			},
		}
		req := newTestRequest("GET", "", "", true)
		rec := httptest.NewRecorder()

		handleListProjects(store)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		body := decodeEnvelope(t, rec)
		list, ok := body.Data.([]any)
		if !ok || len(list) != 0 {
			t.Errorf("data = %+v, want an empty array", body.Data)
		}
	})

	t.Run("success returns the user's projects", func(t *testing.T) {
		var gotUserID uuid.UUID
		store := &fakeProjectStore{
			t: t,
			listByUserFn: func(ctx context.Context, userID uuid.UUID) ([]projects.Project, error) {
				gotUserID = userID
				return []projects.Project{
					{ID: uuid.New(), UserID: userID},
					{ID: uuid.New(), UserID: userID},
				}, nil
			},
		}
		req := newTestRequest("GET", "", "", true)
		rec := httptest.NewRecorder()

		handleListProjects(store)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
		if gotUserID != testUserID {
			t.Errorf("userID passed to store = %v, want %v", gotUserID, testUserID)
		}
		body := decodeEnvelope(t, rec)
		list, ok := body.Data.([]any)
		if !ok || len(list) != 2 {
			t.Errorf("data = %+v, want 2 projects", body.Data)
		}
	})
}

func TestHandleGetProject(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeProjectStore{t: t}
		req := newTestRequest("GET", uuid.New().String(), "", false)
		rec := httptest.NewRecorder()

		handleGetProject(store)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		store := &fakeProjectStore{t: t}
		req := newTestRequest("GET", "not-a-uuid", "", true)
		rec := httptest.NewRecorder()

		handleGetProject(store)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_id" {
			t.Errorf("error code = %+v, want invalid_id", body.Error)
		}
	})

	t.Run("not found", func(t *testing.T) {
		store := &fakeProjectStore{
			t: t,
			getByIDFn: func(ctx context.Context, id, userID uuid.UUID) (projects.Project, error) {
				return projects.Project{}, projects.ErrNotFound
			},
		}
		req := newTestRequest("GET", uuid.New().String(), "", true)
		rec := httptest.NewRecorder()

		handleGetProject(store)(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		id := uuid.New()
		store := &fakeProjectStore{
			t: t,
			getByIDFn: func(ctx context.Context, gotID, userID uuid.UUID) (projects.Project, error) {
				if gotID != id {
					t.Errorf("id = %v, want %v", gotID, id)
				}
				if userID != testUserID {
					t.Errorf("userID = %v, want %v", userID, testUserID)
				}
				return projects.Project{ID: id, UserID: userID}, nil
			},
		}
		req := newTestRequest("GET", id.String(), "", true)
		rec := httptest.NewRecorder()

		handleGetProject(store)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleUpdateProject(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeProjectStore{t: t}
		req := newTestRequest("PATCH", uuid.New().String(), `{"title":"x"}`, false)
		rec := httptest.NewRecorder()

		handleUpdateProject(store)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		store := &fakeProjectStore{t: t}
		req := newTestRequest("PATCH", "not-a-uuid", `{"title":"x"}`, true)
		rec := httptest.NewRecorder()

		handleUpdateProject(store)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_id" {
			t.Errorf("error code = %+v, want invalid_id", body.Error)
		}
	})

	t.Run("missing title", func(t *testing.T) {
		store := &fakeProjectStore{t: t}
		req := newTestRequest("PATCH", uuid.New().String(), `{"description":"x"}`, true)
		rec := httptest.NewRecorder()

		handleUpdateProject(store)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_title" {
			t.Errorf("error code = %+v, want invalid_title", body.Error)
		}
	})

	t.Run("not found", func(t *testing.T) {
		store := &fakeProjectStore{
			t: t,
			updateFn: func(ctx context.Context, id, userID uuid.UUID, title, description string) (projects.Project, error) {
				return projects.Project{}, projects.ErrNotFound
			},
		}
		req := newTestRequest("PATCH", uuid.New().String(), `{"title":"x"}`, true)
		rec := httptest.NewRecorder()

		handleUpdateProject(store)(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		id := uuid.New()
		store := &fakeProjectStore{
			t: t,
			updateFn: func(ctx context.Context, gotID, userID uuid.UUID, title, description string) (projects.Project, error) {
				return projects.Project{ID: gotID, UserID: userID, Title: title, Description: description}, nil
			},
		}
		req := newTestRequest("PATCH", id.String(), `{"title":"new title","description":"new desc"}`, true)
		rec := httptest.NewRecorder()

		handleUpdateProject(store)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleDeleteProject(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeProjectStore{t: t}
		req := newTestRequest("DELETE", uuid.New().String(), "", false)
		rec := httptest.NewRecorder()

		handleDeleteProject(store)(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		store := &fakeProjectStore{t: t}
		req := newTestRequest("DELETE", "not-a-uuid", "", true)
		rec := httptest.NewRecorder()

		handleDeleteProject(store)(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_id" {
			t.Errorf("error code = %+v, want invalid_id", body.Error)
		}
	})

	t.Run("not found", func(t *testing.T) {
		store := &fakeProjectStore{
			t: t,
			deleteFn: func(ctx context.Context, id, userID uuid.UUID) error {
				return projects.ErrNotFound
			},
		}
		req := newTestRequest("DELETE", uuid.New().String(), "", true)
		rec := httptest.NewRecorder()

		handleDeleteProject(store)(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("success", func(t *testing.T) {
		id := uuid.New()
		var gotID, gotUserID uuid.UUID
		store := &fakeProjectStore{
			t: t,
			deleteFn: func(ctx context.Context, id, userID uuid.UUID) error {
				gotID = id
				gotUserID = userID
				return nil
			},
		}
		req := newTestRequest("DELETE", id.String(), "", true)
		rec := httptest.NewRecorder()

		handleDeleteProject(store)(rec, req)

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

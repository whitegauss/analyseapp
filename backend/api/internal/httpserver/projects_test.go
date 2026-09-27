package httpserver

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"analyseapp/api/internal/projects"
	"analyseapp/api/internal/response"
)

// newProjectStore is the default store for the tables below: every method
// fails the test if called.
func newProjectStore(t *testing.T) *fakeProjectStore { return &fakeProjectStore{t: t} }

// wantProject checks the response carries the project the store returned.
func wantProject(id uuid.UUID, title, description string) func(t *testing.T, body response.Envelope) {
	return func(t *testing.T, body response.Envelope) {
		data, _ := body.Data.(map[string]any)
		if data["id"] != id.String() || data["title"] != title || data["description"] != description {
			t.Errorf("data = %+v, want project %v %q %q", body.Data, id, title, description)
		}
	}
}

func TestHandleCreateProject(t *testing.T) {
	createdID := uuid.New()
	const body = `{"title":"my project","description":"desc"}`

	// stores builds the success path: a profile ensured, then a Create that
	// checks it was handed the request's title and description on behalf of
	// the authenticated user -- after the profile, whose row
	// projects.user_id references.
	stores := func(createErr error) func(t *testing.T) *fakeProjectStore {
		return func(t *testing.T) *fakeProjectStore {
			ensured := false
			return &fakeProjectStore{
				t: t,
				ensureProfileFn: func(context.Context, uuid.UUID) error {
					ensured = true
					return nil
				},
				createFn: func(_ context.Context, userID uuid.UUID, title, description string) (projects.Project, error) {
					if !ensured {
						t.Error("Create was called before EnsureProfile")
					}
					if userID != testUserID {
						t.Errorf("userID passed to store = %v, want the authenticated user %v", userID, testUserID)
					}
					if title != "my project" || description != "desc" {
						t.Errorf("title, description passed to store = %q, %q, want the request's", title, description)
					}
					if createErr != nil {
						return projects.Project{}, createErr
					}
					return projects.Project{ID: createdID, UserID: userID, Title: title, Description: description}, nil
				},
			}
		}
	}

	runHandlerCases(t, "POST", []handlerCase[*fakeProjectStore]{
		{name: "unauthenticated", body: body, unauthenticated: true,
			wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "invalid JSON body", body: `not json`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_body"},
		{name: "missing title", body: `{"description":"x"}`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_title"},
		{name: "success returns the created project", body: body, store: stores(nil),
			wantStatus: http.StatusCreated, check: wantProject(createdID, "my project", "desc")},
		{name: "profile cannot be ensured", body: body,
			store: func(t *testing.T) *fakeProjectStore {
				return &fakeProjectStore{t: t, ensureProfileFn: func(context.Context, uuid.UUID) error {
					return errStoreDown
				}}
			},
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
		{name: "store failure", body: body, store: stores(errStoreDown),
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}, newProjectStore, func(s *fakeProjectStore) http.HandlerFunc { return handleCreateProject(s) })
}

func TestHandleListProjects(t *testing.T) {
	listByUser := func(n int, err error) func(t *testing.T) *fakeProjectStore {
		return func(t *testing.T) *fakeProjectStore {
			return &fakeProjectStore{t: t, listByUserFn: func(_ context.Context, userID uuid.UUID) ([]projects.Project, error) {
				if userID != testUserID {
					t.Errorf("userID passed to store = %v, want the authenticated user %v", userID, testUserID)
				}
				if err != nil {
					return nil, err
				}
				list := []projects.Project{}
				for range n {
					list = append(list, projects.Project{ID: uuid.New(), UserID: userID})
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

	runHandlerCases(t, "GET", []handlerCase[*fakeProjectStore]{
		{name: "unauthenticated", unauthenticated: true,
			wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "success returns the user's projects", store: listByUser(2, nil),
			wantStatus: http.StatusOK, check: wantLen(2)},
		// An empty array, not null: the dashboard calls .map() on this.
		{name: "no projects is an empty array", store: listByUser(0, nil),
			wantStatus: http.StatusOK, check: wantLen(0)},
		{name: "store failure", store: listByUser(0, errStoreDown),
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}, newProjectStore, func(s *fakeProjectStore) http.HandlerFunc { return handleListProjects(s) })
}

func TestHandleGetProject(t *testing.T) {
	id := uuid.New()
	getByID := func(err error) func(t *testing.T) *fakeProjectStore {
		return func(t *testing.T) *fakeProjectStore {
			return &fakeProjectStore{t: t, getByIDFn: func(_ context.Context, gotID, userID uuid.UUID) (projects.Project, error) {
				wantStoreCall(t, gotID, id, userID)
				if err != nil {
					return projects.Project{}, err
				}
				return projects.Project{ID: gotID, UserID: userID, Title: "p", Description: "d"}, nil
			}}
		}
	}

	runHandlerCases(t, "GET", []handlerCase[*fakeProjectStore]{
		{name: "unauthenticated", id: id.String(), unauthenticated: true,
			wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "invalid id", id: "not-a-uuid",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "success returns the stored project", id: id.String(), store: getByID(nil),
			wantStatus: http.StatusOK, check: wantProject(id, "p", "d")},
		{name: "not found", id: id.String(), store: getByID(projects.ErrNotFound),
			wantStatus: http.StatusNotFound, wantCode: "not_found", check: wantMessage("project not found")},
		{name: "store failure", id: id.String(), store: getByID(errStoreDown),
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}, newProjectStore, func(s *fakeProjectStore) http.HandlerFunc { return handleGetProject(s) })
}

func TestHandleUpdateProject(t *testing.T) {
	id := uuid.New()
	const body = `{"title":"new title","description":"new desc"}`
	update := func(err error) func(t *testing.T) *fakeProjectStore {
		return func(t *testing.T) *fakeProjectStore {
			return &fakeProjectStore{t: t, updateFn: func(_ context.Context, gotID, userID uuid.UUID, title, description string) (projects.Project, error) {
				wantStoreCall(t, gotID, id, userID)
				if title != "new title" || description != "new desc" {
					t.Errorf("title, description passed to store = %q, %q, want the request's", title, description)
				}
				if err != nil {
					return projects.Project{}, err
				}
				return projects.Project{ID: gotID, UserID: userID, Title: title, Description: description}, nil
			}}
		}
	}

	runHandlerCases(t, "PATCH", []handlerCase[*fakeProjectStore]{
		{name: "unauthenticated", id: id.String(), body: body, unauthenticated: true,
			wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "invalid id", id: "not-a-uuid", body: body,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "missing title", id: id.String(), body: `{"description":"x"}`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_title"},
		{name: "success returns the updated project", id: id.String(), body: body, store: update(nil),
			wantStatus: http.StatusOK, check: wantProject(id, "new title", "new desc")},
		{name: "not found", id: id.String(), body: body, store: update(projects.ErrNotFound),
			wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "store failure", id: id.String(), body: body, store: update(errStoreDown),
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}, newProjectStore, func(s *fakeProjectStore) http.HandlerFunc { return handleUpdateProject(s) })
}

func TestHandleDeleteProject(t *testing.T) {
	id := uuid.New()
	deleteReturning := func(err error) func(t *testing.T) *fakeProjectStore {
		return func(t *testing.T) *fakeProjectStore {
			return &fakeProjectStore{t: t, deleteFn: func(_ context.Context, gotID, userID uuid.UUID) error {
				wantStoreCall(t, gotID, id, userID)
				return err
			}}
		}
	}

	runHandlerCases(t, "DELETE", []handlerCase[*fakeProjectStore]{
		{name: "unauthenticated", id: id.String(), unauthenticated: true,
			wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "invalid id", id: "not-a-uuid",
			wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "success echoes the deleted id", id: id.String(), store: deleteReturning(nil),
			wantStatus: http.StatusOK, check: wantData("id", id.String())},
		{name: "not found", id: id.String(), store: deleteReturning(projects.ErrNotFound),
			wantStatus: http.StatusNotFound, wantCode: "not_found"},
		{name: "store failure", id: id.String(), store: deleteReturning(errStoreDown),
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}, newProjectStore, func(s *fakeProjectStore) http.HandlerFunc { return handleDeleteProject(s) })
}

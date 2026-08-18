package httpserver

import (
	"net/http"

	"analyseapp/api/internal/projects"
	"analyseapp/api/internal/response"
)

type createProjectRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type updateProjectRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// writeProjectError maps a projects.Store error to the right HTTP response:
// 404 for ErrNotFound (never distinguishes "doesn't exist" from "not
// yours", so ownership isn't leaked), 500 otherwise. failedTo completes the
// message "failed to <failedTo>" (e.g. "get project").
func writeProjectError(w http.ResponseWriter, err error, failedTo string) {
	if err == projects.ErrNotFound {
		response.WriteError(w, http.StatusNotFound, "not_found", "project not found")
		return
	}
	response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to "+failedTo)
}

func handleCreateProject(repo projects.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}

		var req createProjectRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}
		if req.Title == "" {
			response.WriteError(w, http.StatusBadRequest, "invalid_title", "title is required")
			return
		}

		if err := repo.EnsureProfile(r.Context(), userID); err != nil {
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to ensure profile")
			return
		}

		p, err := repo.Create(r.Context(), userID, req.Title, req.Description)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create project")
			return
		}

		response.WriteData(w, http.StatusCreated, p)
	}
}

func handleListProjects(repo projects.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}

		list, err := repo.ListByUser(r.Context(), userID)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list projects")
			return
		}

		response.WriteData(w, http.StatusOK, list)
	}
}

func handleGetProject(repo projects.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		id, ok := parseIDParam(w, r)
		if !ok {
			return
		}

		p, err := repo.GetByID(r.Context(), id, userID)
		if err != nil {
			writeProjectError(w, err, "get project")
			return
		}

		response.WriteData(w, http.StatusOK, p)
	}
}

func handleUpdateProject(repo projects.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		id, ok := parseIDParam(w, r)
		if !ok {
			return
		}

		var req updateProjectRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}
		if req.Title == "" {
			response.WriteError(w, http.StatusBadRequest, "invalid_title", "title is required")
			return
		}

		p, err := repo.Update(r.Context(), id, userID, req.Title, req.Description)
		if err != nil {
			writeProjectError(w, err, "update project")
			return
		}

		response.WriteData(w, http.StatusOK, p)
	}
}

func handleDeleteProject(repo projects.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		id, ok := parseIDParam(w, r)
		if !ok {
			return
		}

		if err := repo.Delete(r.Context(), id, userID); err != nil {
			writeProjectError(w, err, "delete project")
			return
		}

		response.WriteData(w, http.StatusOK, map[string]string{"id": id.String()})
	}
}

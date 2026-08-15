package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"analyseapp/api/internal/auth"
	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/response"
)

type createExperimentRequest struct {
	Title   *string        `json:"title"`
	RawData map[string]any `json:"raw_data"`
	Config  map[string]any `json:"config"`
}

type updateConfigRequest struct {
	Config map[string]any `json:"config"`
}

func handleCreateExperiment(repo experiments.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserID(r.Context())
		if !ok {
			response.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authenticated user")
			return
		}

		var req createExperimentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.WriteError(w, http.StatusBadRequest, "invalid_body", "could not parse JSON body")
			return
		}
		if req.Title != nil && *req.Title == "" {
			req.Title = nil
		}
		if req.RawData == nil {
			response.WriteError(w, http.StatusBadRequest, "invalid_raw_data", "raw_data is required")
			return
		}

		if err := repo.EnsureProfile(r.Context(), userID); err != nil {
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to ensure profile")
			return
		}

		e, err := repo.Create(r.Context(), userID, req.Title, req.RawData, req.Config)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create experiment")
			return
		}

		response.WriteData(w, http.StatusCreated, e)
	}
}

func handleListExperiments(repo experiments.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserID(r.Context())
		if !ok {
			response.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authenticated user")
			return
		}

		list, err := repo.ListByUser(r.Context(), userID)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list experiments")
			return
		}

		response.WriteData(w, http.StatusOK, list)
	}
}

func handleGetExperiment(repo experiments.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserID(r.Context())
		if !ok {
			response.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authenticated user")
			return
		}

		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			response.WriteError(w, http.StatusBadRequest, "invalid_id", "id is not a valid UUID")
			return
		}

		e, err := repo.GetByID(r.Context(), id, userID)
		if err != nil {
			if err == experiments.ErrNotFound {
				response.WriteError(w, http.StatusNotFound, "not_found", "experiment not found")
				return
			}
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to get experiment")
			return
		}

		response.WriteData(w, http.StatusOK, e)
	}
}

func handleDeleteExperiment(repo experiments.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserID(r.Context())
		if !ok {
			response.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authenticated user")
			return
		}

		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			response.WriteError(w, http.StatusBadRequest, "invalid_id", "id is not a valid UUID")
			return
		}

		if err := repo.Delete(r.Context(), id, userID); err != nil {
			if err == experiments.ErrNotFound {
				response.WriteError(w, http.StatusNotFound, "not_found", "experiment not found")
				return
			}
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to delete experiment")
			return
		}

		response.WriteData(w, http.StatusOK, map[string]string{"id": id.String()})
	}
}

func handleUpdateExperimentConfig(repo experiments.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.UserID(r.Context())
		if !ok {
			response.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authenticated user")
			return
		}

		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			response.WriteError(w, http.StatusBadRequest, "invalid_id", "id is not a valid UUID")
			return
		}

		var req updateConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.WriteError(w, http.StatusBadRequest, "invalid_body", "could not parse JSON body")
			return
		}
		if req.Config == nil {
			response.WriteError(w, http.StatusBadRequest, "invalid_config", "config is required")
			return
		}

		e, err := repo.UpdateConfig(r.Context(), id, userID, req.Config)
		if err != nil {
			if err == experiments.ErrNotFound {
				response.WriteError(w, http.StatusNotFound, "not_found", "experiment not found")
				return
			}
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to update experiment config")
			return
		}

		response.WriteData(w, http.StatusOK, e)
	}
}

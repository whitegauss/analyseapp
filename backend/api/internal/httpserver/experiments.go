package httpserver

import (
	"net/http"

	"analyseapp/api/internal/cache"
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

type updateRawDataRequest struct {
	RawData map[string]any `json:"raw_data"`
}

func handleCreateExperiment(repo experiments.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}

		var req createExperimentRequest
		if !decodeJSONBody(w, r, &req) {
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
		userID, ok := requireUserID(w, r)
		if !ok {
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
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		id, ok := parseIDParam(w, r)
		if !ok {
			return
		}

		e, err := repo.GetByID(r.Context(), id, userID)
		if err != nil {
			writeExperimentError(w, err, "get experiment")
			return
		}

		response.WriteData(w, http.StatusOK, e)
	}
}

func handleDeleteExperiment(repo experiments.Store) http.HandlerFunc {
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
			writeExperimentError(w, err, "delete experiment")
			return
		}

		response.WriteData(w, http.StatusOK, map[string]string{"id": id.String()})
	}
}

func handleUpdateExperimentConfig(repo experiments.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		id, ok := parseIDParam(w, r)
		if !ok {
			return
		}

		var req updateConfigRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}
		if req.Config == nil {
			response.WriteError(w, http.StatusBadRequest, "invalid_config", "config is required")
			return
		}

		e, err := repo.UpdateConfig(r.Context(), id, userID, req.Config)
		if err != nil {
			writeExperimentError(w, err, "update experiment config")
			return
		}

		response.WriteData(w, http.StatusOK, e)
	}
}

// handleUpdateExperimentRawData replaces an experiment's raw_data wholesale.
// Any cached /analyze results for this experiment are invalidated afterward
// (best-effort, same as the analyze path's cache Set) since a cached result
// keyed only by type/params would otherwise keep serving stale analysis
// output computed from the old data for up to cache.AnalysisTTL.
func handleUpdateExperimentRawData(repo experiments.Store, resultCache cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		id, ok := parseIDParam(w, r)
		if !ok {
			return
		}

		var req updateRawDataRequest
		if !decodeJSONBody(w, r, &req) {
			return
		}
		if req.RawData == nil {
			response.WriteError(w, http.StatusBadRequest, "invalid_raw_data", "raw_data is required")
			return
		}

		e, err := repo.UpdateRawData(r.Context(), id, userID, req.RawData)
		if err != nil {
			writeExperimentError(w, err, "update experiment raw_data")
			return
		}

		_ = resultCache.DeleteByPrefix(r.Context(), cache.AnalysisKeyPrefix(id))

		response.WriteData(w, http.StatusOK, e)
	}
}

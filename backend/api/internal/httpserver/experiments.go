package httpserver

import (
	"net/http"

	"analyseapp/api/internal/cache"
	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/projects"
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

// parseCreateExperimentRequest decodes and validates the body both create
// paths share, writing the 400 itself and returning ok=false. Only the
// project differs between them: the flat path resolves the default one, the
// nested path takes it from the URL.
//
// A title of "" is normalized to nil rather than rejected: the column is
// nullable and the UI sends an empty field for "not named yet", which
// should mean the same as omitting it.
func parseCreateExperimentRequest(w http.ResponseWriter, r *http.Request) (createExperimentRequest, bool) {
	var req createExperimentRequest
	if !decodeJSONBody(w, r, &req) {
		return req, false
	}
	if req.Title != nil && *req.Title == "" {
		req.Title = nil
	}
	if req.RawData == nil {
		response.WriteError(w, http.StatusBadRequest, "invalid_raw_data", "raw_data is required")
		return req, false
	}
	return req, true
}

// handleCreateExperiment serves the flat POST /api/v1/experiments, which
// names no project. Every experiment needs one now that project_id is NOT
// NULL, so the request lands in the user's default project (projects
// .DefaultTitle), created on first use.
//
// The endpoint stays for the same reason the default project exists: the UI
// has no project picker yet (KAN-27), so it has nothing to put in the
// nested POST /api/v1/projects/{id}/experiments below. Whether it is
// retired once the picker lands is decided there.
func handleCreateExperiment(repo experiments.Store, projectRepo projects.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}

		req, ok := parseCreateExperimentRequest(w, r)
		if !ok {
			return
		}

		if err := repo.EnsureProfile(r.Context(), userID); err != nil {
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to ensure profile")
			return
		}

		project, err := projectRepo.EnsureDefault(r.Context(), userID)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to resolve the default project")
			return
		}

		e, err := repo.Create(r.Context(), userID, project.ID, req.Title, req.RawData, req.Config)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to create experiment")
			return
		}

		response.WriteData(w, http.StatusCreated, e)
	}
}

// handleCreateProjectExperiment serves POST
// /api/v1/projects/{id}/experiments: the create path that names its project
// explicitly (PDR.md section 8 -- collections hang off the project, while
// operations on one experiment stay on the flat /experiments/{id} paths,
// since an experiment id is unique on its own).
//
// The project id is not checked here before inserting. Store.Create guards
// the insert with "where exists (select 1 from projects where id = $2 and
// user_id = $1)", so an id belonging to someone else writes nothing and
// comes back as ErrNotFound -- one query, and no window between the check
// and the insert.
func handleCreateProjectExperiment(repo experiments.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		projectID, ok := parseIDParam(w, r)
		if !ok {
			return
		}
		req, ok := parseCreateExperimentRequest(w, r)
		if !ok {
			return
		}

		// No EnsureProfile, unlike the flat path: the insert only succeeds
		// for a project this user already owns, and projects.user_id
		// references profiles, so the row this would create is guaranteed
		// to exist already.
		e, err := repo.Create(r.Context(), userID, projectID, req.Title, req.RawData, req.Config)
		if err != nil {
			writeNestedExperimentError(w, err, "create experiment")
			return
		}

		response.WriteData(w, http.StatusCreated, e)
	}
}

// handleListProjectExperiments serves GET
// /api/v1/projects/{id}/experiments. The cross-project
// GET /api/v1/experiments stays alongside it (PDR.md section 8): choosing a
// copy source and comparing experiments both need every project at once.
func handleListProjectExperiments(repo experiments.Store, projectRepo projects.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := requireUserID(w, r)
		if !ok {
			return
		}
		projectID, ok := parseIDParam(w, r)
		if !ok {
			return
		}

		// The project is read first purely to answer 404 for one that isn't
		// this user's. ListByProject filters on user_id as well, so it can
		// never return someone else's rows -- but it would answer an empty
		// array for a stranger's project id just as it does for an empty
		// one of the user's own, and a 200 there says the id exists.
		if _, err := projectRepo.GetByID(r.Context(), projectID, userID); err != nil {
			writeProjectError(w, err, "get project")
			return
		}

		list, err := repo.ListByProject(r.Context(), projectID, userID)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to list experiments")
			return
		}

		response.WriteData(w, http.StatusOK, list)
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

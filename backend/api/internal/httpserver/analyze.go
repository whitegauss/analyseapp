package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"analyseapp/api/internal/auth"
	"analyseapp/api/internal/cache"
	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/logging"
	"analyseapp/api/internal/response"
	"analyseapp/api/internal/worker"
)

type analyzeRequest struct {
	Type   string         `json:"type"`
	Params map[string]any `json:"params"`
}

// handleAnalyzeExperiment runs an analysis against an experiment's stored
// raw_data. It fetches the experiment (so ownership is enforced the same
// way as the other endpoints), then proxies {type, data, params} to the
// Python worker's /analyze and passes its response straight through --
// the worker already replies with the same {data, error, meta} envelope
// this API uses.
//
// Results are cached in Redis (PDR.md section 7:
// analysis:{experiment_id}:{type}:{params_hash}). A cache hit skips both the
// experiment lookup and the worker call. Cache errors (including an
// unreachable Redis) are treated as a miss -- caching is a performance
// optimization, not a correctness dependency.
func handleAnalyzeExperiment(repo experiments.Store, workerClient worker.Client, resultCache cache.Cache) http.HandlerFunc {
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

		var req analyzeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response.WriteError(w, http.StatusBadRequest, "invalid_body", "could not parse JSON body")
			return
		}
		if req.Type == "" {
			response.WriteError(w, http.StatusBadRequest, "invalid_type", "type is required")
			return
		}

		params := req.Params
		if params == nil {
			params = map[string]any{}
		}

		cacheKey, keyErr := cache.AnalysisKey(id, req.Type, params)
		if keyErr == nil {
			if cached, hit, err := resultCache.Get(r.Context(), cacheKey); err == nil && hit {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Cache", "HIT")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(cached)
				return
			}
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

		workerBody, err := json.Marshal(map[string]any{
			"type":   req.Type,
			"data":   e.RawData,
			"params": params,
		})
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to build analysis request")
			return
		}

		status, respBody, err := workerClient.Analyze(r.Context(), logging.TraceID(r.Context()), workerBody)
		if err != nil {
			response.WriteError(w, http.StatusBadGateway, "worker_unreachable", "failed to reach analysis worker")
			return
		}

		if keyErr == nil && status == http.StatusOK {
			_ = resultCache.Set(r.Context(), cacheKey, respBody, cache.AnalysisTTL)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "MISS")
		w.WriteHeader(status)
		_, _ = w.Write(respBody)
	}
}

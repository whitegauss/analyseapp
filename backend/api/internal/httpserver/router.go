package httpserver

import (
	"net/http"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"analyseapp/api/internal/auth"
	"analyseapp/api/internal/cache"
	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/logging"
	"analyseapp/api/internal/response"
	"analyseapp/api/internal/worker"
)

// NewRouter builds the top-level chi router for the API service.
// dbPool may be nil (e.g. DATABASE_URL not configured yet), in which case
// /readyz reports not-ready and /api/v1 routes fail with 503 rather than
// panicking. jwks gates every /api/v1 route behind Supabase JWT verification
// (PDR.md section 8: browser auth = Supabase OAuth JWT). workerClient and
// resultCache are always non-nil (WORKER_BASE_URL/REDIS_ADDR have
// local-dev defaults); an unreachable Redis degrades resultCache to
// always-miss rather than breaking requests.
func NewRouter(dbPool *pgxpool.Pool, jwks keyfunc.Keyfunc, workerClient worker.Client, resultCache cache.Cache) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(logging.Middleware)

	r.Get("/healthz", handleHealthz)
	r.Get("/readyz", handleReadyz(dbPool))

	if jwks != nil {
		r.Route("/api/v1", func(r chi.Router) {
			r.Use(auth.Middleware(jwks))

			if dbPool != nil {
				repo := experiments.NewRepository(dbPool)
				r.Post("/experiments", handleCreateExperiment(repo))
				r.Get("/experiments", handleListExperiments(repo))
				r.Get("/experiments/{id}", handleGetExperiment(repo))
				r.Delete("/experiments/{id}", handleDeleteExperiment(repo))
				r.Patch("/experiments/{id}/config", handleUpdateExperimentConfig(repo))
				r.Post("/experiments/{id}/analyze", handleAnalyzeExperiment(repo, workerClient, resultCache))
			}
			// convert (PDR.md section 8) lands here in follow-up work.
		})
	}

	return r
}

// handleHealthz is a pure liveness check: it never touches dependencies, so
// it stays fast and reports the process itself is running.
func handleHealthz(w http.ResponseWriter, r *http.Request) {
	response.WriteData(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReadyz is a readiness check: it pings the database on every call so
// it reflects whether the service can currently serve DB-backed requests.
func handleReadyz(dbPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if dbPool == nil {
			response.WriteError(w, http.StatusServiceUnavailable, "db_not_configured", "DATABASE_URL is not set")
			return
		}
		if err := dbPool.Ping(r.Context()); err != nil {
			response.WriteError(w, http.StatusServiceUnavailable, "db_unreachable", err.Error())
			return
		}
		response.WriteData(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

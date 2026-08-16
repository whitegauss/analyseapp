package httpserver

import (
	"net/http"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"

	"analyseapp/api/internal/auth"
	"analyseapp/api/internal/cache"
	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/logging"
	"analyseapp/api/internal/metrics"
	"analyseapp/api/internal/response"
	"analyseapp/api/internal/worker"
)

// apiRateLimit caps each client IP to this many /api/v1 requests per
// apiRateLimitWindow. A generous baseline for a small app: well above normal
// UI usage (a page load fires a handful of requests), but low enough to
// blunt an abusive script hammering the (Worker-calling, DB-hitting)
// /analyze endpoint.
const (
	apiRateLimit       = 100
	apiRateLimitWindow = time.Minute
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
	r.Use(metrics.Middleware)
	r.Use(securityHeaders)
	// Resolves the client IP from the TCP connection, not from a
	// client-supplied header -- correct as long as this service is exposed
	// directly (docker-compose maps its port straight to the host, no
	// reverse proxy in front of it today). If a reverse proxy is added
	// later, switch to middleware.ClientIPFromXFF/ClientIPFromHeader with
	// its trusted CIDR/hop count, or every client behind it would share one
	// rate-limit bucket.
	r.Use(middleware.ClientIPFromRemoteAddr)

	r.Get("/healthz", handleHealthz)
	r.Get("/readyz", handleReadyz(dbPool))
	r.Handle("/metrics", metrics.Handler())

	if jwks != nil {
		r.Route("/api/v1", func(r chi.Router) {
			r.Use(httprate.LimitBy(apiRateLimit, apiRateLimitWindow,
				func(r *http.Request) (string, error) {
					return httprate.CanonicalizeIP(middleware.GetClientIP(r.Context())), nil
				},
				httprate.WithLimitHandler(handleRateLimited),
			))
			r.Use(auth.Middleware(jwks))

			if dbPool != nil {
				repo := experiments.NewRepository(dbPool)
				r.Post("/experiments", handleCreateExperiment(repo))
				r.Get("/experiments", handleListExperiments(repo))
				r.Get("/experiments/{id}", handleGetExperiment(repo))
				r.Delete("/experiments/{id}", handleDeleteExperiment(repo))
				r.Patch("/experiments/{id}/config", handleUpdateExperimentConfig(repo))
				r.Patch("/experiments/{id}/raw_data", handleUpdateExperimentRawData(repo, resultCache))
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

// handleRateLimited responds in the same {data, error, meta} envelope as
// every other endpoint, instead of httprate's default plain-text body.
func handleRateLimited(w http.ResponseWriter, r *http.Request) {
	response.WriteError(w, http.StatusTooManyRequests, "rate_limited", "too many requests, please slow down")
}

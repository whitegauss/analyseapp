package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"analyseapp/api/internal/logging"
)

// NewRouter builds the top-level chi router for the API service.
// At this stage it only exposes a health check; feature routes (experiments,
// analyze, convert per PDR.md section 8) are added in later work.
func NewRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Recoverer)
	r.Use(logging.Middleware)

	r.Get("/healthz", handleHealthz)

	r.Route("/api/v1", func(r chi.Router) {
		// Feature routes (experiments, analyze, convert) land here in
		// follow-up work; the group is registered now so path prefixing
		// stays consistent with PDR.md section 8.
	})

	return r
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	WriteData(w, http.StatusOK, map[string]string{"status": "ok"})
}

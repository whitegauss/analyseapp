package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMiddleware_RecordsRoutePatternNotRawPath(t *testing.T) {
	r := chi.NewRouter()
	r.Use(Middleware)
	r.Get("/experiments/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/experiments/abc-123", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	metricsRec := httptest.NewRecorder()
	Handler().ServeHTTP(metricsRec, httptest.NewRequest("GET", "/metrics", nil))
	body := metricsRec.Body.String()

	if !strings.Contains(body, `route="/experiments/{id}"`) {
		t.Errorf("metrics output missing route label with the matched pattern, got:\n%s", body)
	}
	if strings.Contains(body, "abc-123") {
		t.Errorf("metrics output leaked the raw path parameter (unbounded label cardinality), got:\n%s", body)
	}
}

func TestHandler_ServesPrometheusExpositionFormat(t *testing.T) {
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain (Prometheus exposition format)", ct)
	}
}

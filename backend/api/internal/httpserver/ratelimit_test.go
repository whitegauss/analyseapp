package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
)

// TestRateLimiting exercises the exact middleware stack router.go wires up
// for /api/v1 (client-IP resolution + httprate + the envelope-shaped 429
// handler), just with a small enough limit to trigger within a test.
func TestRateLimiting(t *testing.T) {
	r := chi.NewRouter()
	r.Use(middleware.ClientIPFromRemoteAddr)
	r.Use(httprate.LimitBy(2, time.Minute,
		func(r *http.Request) (string, error) {
			return httprate.CanonicalizeIP(middleware.GetClientIP(r.Context())), nil
		},
		httprate.WithLimitHandler(handleRateLimited),
	))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	requestFrom := func(remoteAddr string) *http.Request {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = remoteAddr
		return req
	}

	for i := range 2 {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, requestFrom("203.0.113.5:12345"))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i+1, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, requestFrom("203.0.113.5:12345"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd request: status = %d, want 429", rec.Code)
	}
	body := decodeEnvelope(t, rec)
	if body.Error == nil || body.Error.Code != "rate_limited" {
		t.Errorf("error = %+v, want code rate_limited", body.Error)
	}

	// A different client IP has its own, unaffected bucket.
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, requestFrom("198.51.100.9:54321"))
	if rec.Code != http.StatusOK {
		t.Errorf("different IP: status = %d, want 200", rec.Code)
	}
}

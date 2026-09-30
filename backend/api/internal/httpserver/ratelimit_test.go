package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"analyseapp/api/internal/auth"
	"analyseapp/api/internal/response"
)

// fakeAuthenticate stands in for auth.Middleware: the user comes from a
// test header instead of a verified JWT, and a request without one is
// turned away with the same 401 the real middleware gives.
func fakeAuthenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(r.Header.Get("X-Test-User"))
		if err != nil {
			response.WriteError(w, http.StatusUnauthorized, "unauthorized", "no user")
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithUserID(r.Context(), id)))
	})
}

// TestRateLimiting exercises the middleware stack router.go wires up for
// /api/v1 -- apiMiddlewares, with the envelope-shaped 429 handler -- just
// with a limit small enough to trigger within a test.
func TestRateLimiting(t *testing.T) {
	alice, bob := uuid.New(), uuid.New()

	newRouter := func() http.Handler {
		r := chi.NewRouter()
		r.Use(apiMiddlewares(fakeAuthenticate, 2, time.Minute)...)
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		return r
	}
	// Every request comes from the same address, as they all do in
	// production: it is always the Next.js server calling. Only the user
	// differs.
	send := func(h http.Handler, user string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "172.18.0.5:40000"
		if user != "" {
			req.Header.Set("X-Test-User", user)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("a user's own requests are limited", func(t *testing.T) {
		h := newRouter()
		for i := range 2 {
			if rec := send(h, alice.String()); rec.Code != http.StatusOK {
				t.Fatalf("request %d: status = %d, want 200", i+1, rec.Code)
			}
		}

		rec := send(h, alice.String())
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("3rd request: status = %d, want 429", rec.Code)
		}
		body := decodeEnvelope(t, rec)
		if body.Error == nil || body.Error.Code != "rate_limited" {
			t.Errorf("error = %+v, want code rate_limited", body.Error)
		}
		// httprate's headers survive the custom limit handler, so a client
		// can tell when to come back instead of retrying blind. Pinned
		// because swapping the handler is exactly the change that could
		// drop them.
		if got := rec.Header().Get("Retry-After"); got != "60" {
			t.Errorf("Retry-After = %q, want 60 (the window, in seconds)", got)
		}
		if got := rec.Header().Get("X-RateLimit-Remaining"); got != "0" {
			t.Errorf("X-RateLimit-Remaining = %q, want 0", got)
		}
	})

	// The reason for KAN-89: counted by address, one busy user would have
	// locked everyone else out.
	t.Run("another user from the same address has their own allowance", func(t *testing.T) {
		h := newRouter()
		for range 3 {
			send(h, alice.String())
		}
		if rec := send(h, bob.String()); rec.Code != http.StatusOK {
			t.Errorf("other user: status = %d, want 200", rec.Code)
		}
	})

	// Authentication runs first: an unauthenticated request is a 401, not a
	// 429, however many there are, and spends nobody's allowance.
	t.Run("unauthenticated requests are 401 and use no allowance", func(t *testing.T) {
		h := newRouter()
		for i := range 5 {
			if rec := send(h, ""); rec.Code != http.StatusUnauthorized {
				t.Fatalf("unauthenticated request %d: status = %d, want 401", i+1, rec.Code)
			}
		}
		for i := range 2 {
			if rec := send(h, alice.String()); rec.Code != http.StatusOK {
				t.Errorf("request %d after the 401s: status = %d, want 200", i+1, rec.Code)
			}
		}
	})
}

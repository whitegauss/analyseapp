package metrics

import (
	"net/http"
	"net/http/httptest"
	"strconv"
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

// scrape returns the current /metrics exposition text.
func scrape(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	return rec.Body.String()
}

// counterValue reads one sample's value out of the exposition text. The
// registry is process-global and shared with the other tests in this package,
// so cases here compare before/after rather than absolute counts.
func counterValue(t *testing.T, body, sample string) float64 {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		name, value, found := strings.Cut(line, " ")
		if !found || name != sample {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			t.Fatalf("parse %q: %v", line, err)
		}
		return v
	}
	return 0
}

// A panicking handler is the case an alert on status=~"5.." exists for, and
// it was the one case that never reached the metrics at all (KAN-67).
func TestMiddlewareRecordsAPanickingHandler(t *testing.T) {
	const route = "/panics/{id}"
	requests := `http_requests_total{method="GET",route="` + route + `",status="500"}`
	panics := `http_panics_total{method="GET",route="` + route + `"}`

	before := scrape(t)
	wantRequests := counterValue(t, before, requests) + 1
	wantPanics := counterValue(t, before, panics) + 1

	r := chi.NewRouter()
	r.Use(Middleware)
	r.Get(route, func(http.ResponseWriter, *http.Request) { panic("boom") })

	// Recoverer is not in this chain: the panic has to leave Middleware for
	// the real router's Recoverer, which sits outside it, to still see it.
	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic did not reach the caller; Recoverer would never see it")
			}
		}()
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/panics/abc", nil))
	}()

	after := scrape(t)
	if got := counterValue(t, after, requests); got != wantRequests {
		t.Errorf("%s = %v, want %v", requests, got, wantRequests)
	}
	if got := counterValue(t, after, panics); got != wantPanics {
		t.Errorf("%s = %v, want %v", panics, got, wantPanics)
	}
	// The latency of a failed request is still latency, and a histogram that
	// silently drops its worst samples misreports the ones it keeps.
	durations := `http_request_duration_seconds_count{method="GET",route="` + route + `"}`
	if got := counterValue(t, after, durations); got != 1 {
		t.Errorf("%s = %v, want 1", durations, got)
	}
}

// The status label has to be the one the client actually saw. A handler that
// panics before writing anything has sent nothing, so the statusWriter's
// initial 200 would be a fiction -- Recoverer is about to send a 500.
func TestMiddlewareRecordsTheStatusTheClientSaw(t *testing.T) {
	tests := []struct {
		name, route string
		handler     http.HandlerFunc
		panics      bool
		wantStatus  string
	}{
		{name: "an explicit status is recorded", route: "/status/explicit", wantStatus: "404",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(404) }},
		// net/http sends an implicit 200 on the first Write, so a handler that
		// never calls WriteHeader still gave the client a 200.
		{name: "a bare Write counts as the implicit 200", route: "/status/write", wantStatus: "200",
			handler: func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }},
		{name: "a panic before anything is written is a 500", route: "/status/panic", wantStatus: "500", panics: true,
			handler: func(http.ResponseWriter, *http.Request) { panic("boom") }},
		// Here the header is already on the wire; Recoverer cannot change it,
		// so the status the client saw is the one to record.
		{name: "a panic after the header keeps the status the client saw", route: "/status/panic-after", wantStatus: "201", panics: true,
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(201); panic("boom") }},
		// A handler that writes a body after WriteHeader must not have its
		// status overwritten by the implicit-200 bookkeeping in Write.
		{name: "a body written after WriteHeader does not reset the status", route: "/status/both", wantStatus: "503",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503); _, _ = w.Write([]byte("nope")) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sample := `http_requests_total{method="GET",route="` + tt.route + `",status="` + tt.wantStatus + `"}`
			want := counterValue(t, scrape(t), sample) + 1

			r := chi.NewRouter()
			r.Use(Middleware)
			r.Get(tt.route, tt.handler)

			func() {
				defer func() {
					if rec := recover(); (rec != nil) != tt.panics {
						t.Errorf("recover() = %v, want a panic: %v", rec, tt.panics)
					}
				}()
				r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", tt.route, nil))
			}()

			if got := counterValue(t, scrape(t), sample); got != want {
				t.Errorf("%s = %v, want %v (the status recorded was not the one the client saw)", sample, got, want)
			}
		})
	}
}

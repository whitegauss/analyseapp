// Package metrics exposes Prometheus metrics for the API service. There is
// no Prometheus server in this project yet (see PDR.md section 11, "監視・
// アラート体制" is still undecided) -- this only publishes /metrics so a
// future scraper can be pointed at it. In a real deployment /metrics should
// be firewalled off from the public internet (it's unauthenticated, same as
// /healthz and /readyz).
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"analyseapp/api/internal/response"
)

var (
	requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests processed, labeled by method, route pattern, and status code.",
		},
		[]string{"method", "route", "status"},
	)

	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds, labeled by method and route pattern.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)

	// panicsTotal counts requests whose handler panicked. Those are usually
	// counted in http_requests_total as status 500 too -- that is what an
	// alert on status=~"5.." needs -- so this exists to tell them apart from
	// a 500 the code chose to return. It is also the only signal for a panic
	// raised after the response header was already sent, which keeps the
	// status the client saw and so never reaches a 5xx alert.
	panicsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_panics_total",
			Help: "Total HTTP requests whose handler panicked, labeled by method and route pattern.",
		},
		[]string{"method", "route"},
	)

	// AnalysisCacheResultsTotal counts /analyze requests by cache outcome
	// (hit or miss) -- the same signal already reported per-request via the
	// X-Cache response header, aggregated here so cache effectiveness is
	// visible over time without grepping logs.
	AnalysisCacheResultsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "analysis_cache_results_total",
			Help: "Total /analyze requests by cache result.",
		},
		[]string{"result"},
	)
)

// Handler exposes the Prometheus text-format metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}

// Middleware records request count and latency for every request that
// passes through it. Route labels use chi's matched route *pattern* (e.g.
// "/api/v1/experiments/{id}"), not the raw request path, so that path
// parameters like UUIDs don't cause unbounded label cardinality.
//
// Recording happens in a defer, so a panicking handler is counted rather
// than skipped. Written after next.ServeHTTP instead, the panic unwinds
// straight past it: middleware.Recoverer sits outside this one and turns
// the panic into a 500 the client sees, while the metrics show nothing at
// all -- exactly the requests an alert on status=~"5.." is meant to catch
// (KAN-67). The panic is re-raised afterwards so Recoverer still handles
// it; this middleware only observes.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := response.NewStatusWriter(w)

		defer func() {
			rec := recover()
			route := "unmatched"
			if rctx := chi.RouteContext(r.Context()); rctx != nil {
				if pattern := rctx.RoutePattern(); pattern != "" {
					route = pattern
				}
			}

			// An ErrAbortHandler panic is a deliberate abort, not a fault:
			// net/http and chi's Recoverer both let it through untouched, so
			// no 500 is ever sent and there is no panic to report. Compared
			// by identity because that is the test both of them apply -- a
			// wrapped value is not an abort to them either, so it must not
			// be one here. It is still re-raised: only the accounting
			// changes, never the control flow.
			fault := rec
			if fault == http.ErrAbortHandler {
				fault = nil
			}
			if fault != nil {
				panicsTotal.WithLabelValues(r.Method, route).Inc()
			}

			requestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(sw.ObservedStatus(fault))).Inc()
			requestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())

			if rec != nil {
				panic(rec)
			}
		}()

		next.ServeHTTP(sw, r)
	})
}

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

	// PanicsTotal counts requests whose handler panicked. Those are also
	// counted in http_requests_total as status 500 -- that is what an alert
	// on status=~"5.." needs -- so this exists to tell them apart from a 500
	// the code chose to return.
	PanicsTotal = promauto.NewCounterVec(
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
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			rec := recover()
			route := "unmatched"
			if rctx := chi.RouteContext(r.Context()); rctx != nil {
				if pattern := rctx.RoutePattern(); pattern != "" {
					route = pattern
				}
			}
			if rec != nil {
				PanicsTotal.WithLabelValues(r.Method, route).Inc()
				// Recoverer will send the 500 -- unless the handler had
				// already written a header, in which case the status the
				// client saw is the one already recorded.
				if !sw.wroteHeader {
					sw.status = http.StatusInternalServerError
				}
			}

			requestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(sw.status)).Inc()
			requestDuration.WithLabelValues(r.Method, route).Observe(time.Since(start).Seconds())

			if rec != nil {
				panic(rec)
			}
		}()

		next.ServeHTTP(sw, r)
	})
}

// statusWriter remembers the status the client was actually sent. status
// starts at 200 because a handler that only calls Write sends one
// implicitly; wroteHeader separates that from a handler that panicked
// before sending anything, whose 200 would be a fiction.
type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sw *statusWriter) WriteHeader(status int) {
	if !sw.wroteHeader {
		sw.status = status
		sw.wroteHeader = true
	}
	sw.ResponseWriter.WriteHeader(status)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	sw.wroteHeader = true
	return sw.ResponseWriter.Write(b)
}

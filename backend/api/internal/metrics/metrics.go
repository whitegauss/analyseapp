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
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		duration := time.Since(start).Seconds()

		route := "unmatched"
		if rctx := chi.RouteContext(r.Context()); rctx != nil {
			if pattern := rctx.RoutePattern(); pattern != "" {
				route = pattern
			}
		}

		requestsTotal.WithLabelValues(r.Method, route, strconv.Itoa(sw.status)).Inc()
		requestDuration.WithLabelValues(r.Method, route).Observe(duration)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(status int) {
	sw.status = status
	sw.ResponseWriter.WriteHeader(status)
}

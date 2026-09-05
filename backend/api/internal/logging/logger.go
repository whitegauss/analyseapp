// Package logging configures structured JSON logging and per-request trace IDs.
package logging

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type ctxKey string

const traceIDKey ctxKey = "trace_id"

// Init configures the global zerolog logger to emit JSON to stdout.
func Init() {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
}

// TraceID extracts the trace ID stored in the request context, if any.
func TraceID(ctx context.Context) string {
	if v, ok := ctx.Value(traceIDKey).(string); ok {
		return v
	}
	return ""
}

// Middleware assigns a trace ID to every request (reusing an inbound
// X-Trace-Id header when present so it can be propagated from upstream
// callers or down to the Python worker), and logs the request as JSON.
//
// The log line is written from a defer, so a panicking handler still leaves
// a record. Written after next.ServeHTTP instead, the panic unwinds past it
// and the request the operator most needs to find -- the one the client saw
// as a 500 -- is the one with no log line and no trace ID to search for
// (KAN-67). The panic is re-raised afterwards so middleware.Recoverer, which
// sits outside this one, still handles it.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		traceID := r.Header.Get("X-Trace-Id")
		if traceID == "" {
			traceID = uuid.NewString()
		}
		ctx := context.WithValue(r.Context(), traceIDKey, traceID)
		w.Header().Set("X-Trace-Id", traceID)

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		defer func() {
			rec := recover()
			event := log.Info()
			if rec != nil {
				// Recoverer will send the 500 -- unless the handler had
				// already written a header, in which case the status the
				// client saw is the one already recorded.
				if !sw.wroteHeader {
					sw.status = http.StatusInternalServerError
				}
				event = log.Error().Interface("panic", rec)
			}

			event.
				Str("trace_id", traceID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", sw.status).
				Dur("duration", time.Since(start)).
				Msg("request handled")

			if rec != nil {
				panic(rec)
			}
		}()

		next.ServeHTTP(sw, r.WithContext(ctx))
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

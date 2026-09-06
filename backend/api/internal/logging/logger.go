// Package logging configures structured JSON logging and per-request trace IDs.
package logging

import (
	"context"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"analyseapp/api/internal/response"
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

		sw := response.NewStatusWriter(w)

		defer func() {
			rec := recover()

			// An ErrAbortHandler panic is a deliberate abort, not a fault:
			// net/http and chi's Recoverer both let it through untouched, so
			// no 500 is ever sent and there is nothing to report as an
			// error. Compared by identity because that is the test both of
			// them apply -- a wrapped value is not an abort to them either,
			// so it must not be one here. It is still re-raised: only the
			// log level and status change, never the control flow.
			fault := rec
			if fault == http.ErrAbortHandler {
				fault = nil
			}

			event := log.Info()
			if fault != nil {
				// The panic value alone says what broke but not where.
				// debug.Stack() called from the recovering defer still
				// unwinds through the panicking frames, so the handler that
				// raised it is in here -- carried on the same line as the
				// trace ID, which is what makes it findable later.
				event = log.Error().
					Interface("panic", fault).
					Bytes("stack", debug.Stack())
			}

			event.
				Str("trace_id", traceID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", sw.ObservedStatus(fault)).
				Dur("duration", time.Since(start)).
				Msg("request handled")

			if rec != nil {
				panic(rec)
			}
		}()

		next.ServeHTTP(sw, r.WithContext(ctx))
	})
}

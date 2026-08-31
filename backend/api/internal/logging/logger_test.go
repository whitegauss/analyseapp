package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func decodeEntry(t *testing.T, line []byte) map[string]any {
	t.Helper()

	var got map[string]any
	if err := json.Unmarshal(line, &got); err != nil {
		t.Fatalf("decode log line %q: %v", line, err)
	}
	return got
}

// foreignKey shares this package's context-key value but not its type.
type foreignKey string

func TestTraceIDWithoutAValueInTheContext(t *testing.T) {
	// A call made outside a request carries a background context and must
	// degrade to "no trace id"; nor may a foreign key of the same name match.
	for _, ctx := range []context.Context{
		context.Background(),
		context.WithValue(context.Background(), foreignKey("trace_id"), "impostor"),
	} {
		if got := TraceID(ctx); got != "" {
			t.Errorf("TraceID = %q, want an empty string", got)
		}
	}
}

func TestMiddleware(t *testing.T) {
	tests := []struct {
		name, inboundTraceID string
		// handlerStatus is what the handler writes; 0 means it never writes one.
		handlerStatus, wantStatus int
	}{
		// Upstream callers and the worker share one trace id, so an inbound
		// header has to survive rather than be reassigned.
		{name: "an inbound X-Trace-Id is reused", inboundTraceID: "upstream-1", handlerStatus: 200, wantStatus: 200},
		{name: "a missing X-Trace-Id is generated", handlerStatus: 200, wantStatus: 200},
		{name: "a downstream status is recorded", handlerStatus: 404, wantStatus: 404},
		// net/http implies 200 when a handler only writes a body; statusWriter
		// must report the same rather than a zero status.
		{name: "a handler that never calls WriteHeader is logged as 200", wantStatus: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Swap the global logger for a buffer, and put it back after.
			buf := &bytes.Buffer{}
			saved := log.Logger
			log.Logger = zerolog.New(buf)
			t.Cleanup(func() { log.Logger = saved })

			var seen string
			h := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = TraceID(r.Context())
				if tt.handlerStatus != 0 {
					w.WriteHeader(tt.handlerStatus)
				}
				_, _ = w.Write([]byte("body"))
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/experiments", nil)
			if tt.inboundTraceID != "" {
				req.Header.Set("X-Trace-Id", tt.inboundTraceID)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			// The handler, the response header and the log line must agree on
			// the trace id, or a log line cannot be tied back to a response.
			if seen == "" || (tt.inboundTraceID != "" && seen != tt.inboundTraceID) {
				t.Fatalf("TraceID(r.Context()) in the handler = %q, want a non-empty id (%q if inbound)", seen, tt.inboundTraceID)
			}
			if got := rec.Header().Get("X-Trace-Id"); got != seen {
				t.Errorf("X-Trace-Id response header = %q, want %q", got, seen)
			}
			if rec.Code != tt.wantStatus {
				t.Errorf("response status = %d, want %d", rec.Code, tt.wantStatus)
			}

			entry := decodeEntry(t, buf.Bytes())
			if entry["trace_id"] != seen || entry["method"] != http.MethodGet ||
				entry["path"] != "/api/v1/experiments" || entry["status"] != float64(tt.wantStatus) ||
				entry["message"] != "request handled" || entry["duration"] == nil {
				t.Errorf("log entry = %v, want trace_id %q, GET /api/v1/experiments, status %d and a duration",
					entry, seen, tt.wantStatus)
			}
		})
	}
}

func TestInitWritesJSONToStdout(t *testing.T) {
	// Init replaces process-global state -- zerolog's time format and the
	// logger every package logs through -- so both are put back afterwards,
	// and os.Stdout is redirected to read back what Init's logger emits.
	savedLogger, savedFormat, savedStdout := log.Logger, zerolog.TimeFieldFormat, os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		log.Logger, zerolog.TimeFieldFormat, os.Stdout = savedLogger, savedFormat, savedStdout
	})

	Init()
	log.Info().Str("k", "v").Msg("hello")
	_ = w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}

	entry := decodeEntry(t, out)
	if entry["message"] != "hello" || entry["k"] != "v" || entry["level"] != "info" || entry["time"] == nil {
		t.Errorf("log entry = %v, want the message, field, level and an RFC3339 timestamp", entry)
	}
}

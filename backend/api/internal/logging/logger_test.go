package logging

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// logLine is the subset of the JSON log record these tests assert on.
type logLine struct {
	Level   string `json:"level"`
	TraceID string `json:"trace_id"`
	Method  string `json:"method"`
	Path    string `json:"path"`
	Status  int    `json:"status"`
	Panic   string `json:"panic"`
	Stack   string `json:"stack"`
	Message string `json:"message"`
}

// serve runs handler behind Middleware and returns the response together with
// the single log line it emitted. The global logger is swapped for a buffer
// and restored, since Middleware logs through the package-level log.Logger.
func serve(t *testing.T, req *http.Request, handler http.HandlerFunc) (*httptest.ResponseRecorder, logLine, bool) {
	t.Helper()

	var buf bytes.Buffer
	saved := log.Logger
	log.Logger = zerolog.New(&buf)
	t.Cleanup(func() { log.Logger = saved })

	rec := httptest.NewRecorder()
	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		Middleware(handler).ServeHTTP(rec, req)
	}()

	if buf.Len() == 0 {
		t.Fatal("no log line was written")
	}
	var line logLine
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("parse log line %q: %v", buf.String(), err)
	}
	return rec, line, panicked
}

func TestMiddlewareLogsTheRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/experiments", nil)
	rec, line, panicked := serve(t, req, func(w http.ResponseWriter, r *http.Request) {
		// The handler sees the trace ID through the context, which is how
		// analyze.go forwards it to the worker.
		if TraceID(r.Context()) == "" {
			t.Error("TraceID(ctx) is empty inside the handler")
		}
		w.WriteHeader(http.StatusCreated)
	})

	if panicked {
		t.Fatal("the handler did not panic but the middleware did")
	}
	want := logLine{Level: "info", TraceID: line.TraceID, Method: "GET",
		Path: "/api/v1/experiments", Status: 201, Message: "request handled"}
	if line != want {
		t.Errorf("log line = %+v, want %+v", line, want)
	}
	if line.TraceID == "" {
		t.Error("trace_id is empty, want a generated one")
	}
	if got := rec.Header().Get("X-Trace-Id"); got != line.TraceID {
		t.Errorf("X-Trace-Id header = %q, want it to match the logged %q", got, line.TraceID)
	}
}

func TestMiddlewareReusesAnInboundTraceID(t *testing.T) {
	// Propagated from an upstream caller, so one ID spans the whole chain.
	req := httptest.NewRequest("GET", "/healthz", nil)
	req.Header.Set("X-Trace-Id", "upstream-123")

	rec, line, _ := serve(t, req, func(w http.ResponseWriter, r *http.Request) {
		if got := TraceID(r.Context()); got != "upstream-123" {
			t.Errorf("TraceID(ctx) = %q, want the inbound id", got)
		}
	})

	if line.TraceID != "upstream-123" {
		t.Errorf("logged trace_id = %q, want the inbound id", line.TraceID)
	}
	if got := rec.Header().Get("X-Trace-Id"); got != "upstream-123" {
		t.Errorf("X-Trace-Id header = %q, want the inbound id", got)
	}
}

// The request an operator most needs to find is the one that failed, and a
// panicking handler used to leave no line at all: the log was written after
// next.ServeHTTP, which the panic unwound straight past (KAN-67).
func TestMiddlewareLogsAPanickingHandler(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/experiments/abc/analyze", nil)
	req.Header.Set("X-Trace-Id", "trace-boom")

	_, line, panicked := serve(t, req, func(http.ResponseWriter, *http.Request) {
		panic("worker: HTTP client not configured")
	})

	// Re-raised, so middleware.Recoverer -- which sits outside this one --
	// still turns it into the 500 the client gets.
	if !panicked {
		t.Error("the panic did not reach the caller; Recoverer would never see it")
	}
	// The panic value says what broke; the stack says where. Both have to be
	// on the line carrying the trace ID, or the trace ID leads nowhere.
	if !strings.Contains(line.Stack, "logging.TestMiddlewareLogsAPanickingHandler") {
		t.Errorf("stack = %q, want it to name the frame that panicked", line.Stack)
	}
	line.Stack = ""

	want := logLine{Level: "error", TraceID: "trace-boom", Method: "POST",
		Path: "/api/v1/experiments/abc/analyze", Status: 500,
		Panic: "worker: HTTP client not configured", Message: "request handled"}
	if line != want {
		t.Errorf("log line = %+v, want %+v", line, want)
	}
}

// http.ErrAbortHandler is how a handler says "stop, drop the connection" on
// purpose. net/http and chi's Recoverer both let it through untouched -- no
// 500 is ever sent -- so logging it as an error would report an outage that
// did not happen.
func TestMiddlewareDoesNotLogADeliberateAbortAsAnError(t *testing.T) {
	_, line, panicked := serve(t, httptest.NewRequest("GET", "/stream", nil),
		func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })

	// Still re-raised: only the level and status change. net/http relies on
	// receiving it to drop the connection quietly.
	if !panicked {
		t.Error("http.ErrAbortHandler was swallowed; net/http needs it to abort the connection")
	}
	if line.Level != "info" {
		t.Errorf("level = %q, want info (an abort is not a fault)", line.Level)
	}
	if line.Status != 200 {
		t.Errorf("status = %d, want 200 (an abort must not be logged as a 500)", line.Status)
	}
	if line.Panic != "" || line.Stack != "" {
		t.Errorf("panic/stack = %q/%q, want both empty", line.Panic, line.Stack)
	}
}

func TestMiddlewareLogsTheStatusTheClientSaw(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int
	}{
		// net/http sends an implicit 200 on the first Write, so a handler that
		// never calls WriteHeader still gave the client a 200.
		{name: "a bare Write is the implicit 200", wantStatus: 200,
			handler: func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) }},
		// Nothing was sent, so the initial 200 would be a fiction.
		{name: "a panic before anything is written is a 500", wantStatus: 500,
			handler: func(http.ResponseWriter, *http.Request) { panic("boom") }},
		// The header is already on the wire and Recoverer cannot change it.
		{name: "a panic after the header keeps the status the client saw", wantStatus: 201,
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(201); panic("boom") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, line, _ := serve(t, httptest.NewRequest("GET", "/x", nil), tt.handler)
			if line.Status != tt.wantStatus {
				t.Errorf("logged status = %d, want %d", line.Status, tt.wantStatus)
			}
		})
	}
}

func TestTraceIDWithoutMiddleware(t *testing.T) {
	// Callers outside a request (or before the middleware ran) get "", not a
	// panic: analyze.go passes the result straight to the worker client,
	// which omits the header when it is empty.
	if got := TraceID(httptest.NewRequest("GET", "/x", nil).Context()); got != "" {
		t.Errorf("TraceID = %q, want an empty string", got)
	}
}

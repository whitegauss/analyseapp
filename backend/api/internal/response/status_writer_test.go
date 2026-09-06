package response

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// ObservedStatus is where logging and metrics agree on what a request's
// status was, so the cases they both depend on are pinned here once.
func TestStatusWriterObservedStatus(t *testing.T) {
	panicked := "boom"

	tests := []struct {
		name       string
		write      func(w http.ResponseWriter)
		recovered  any
		wantStatus int
	}{
		{name: "an explicit status is kept", wantStatus: 404,
			write: func(w http.ResponseWriter) { w.WriteHeader(404) }},
		// net/http sends an implicit 200 on the first Write, so a handler
		// that never calls WriteHeader still gave the client a 200.
		{name: "a bare Write is the implicit 200", wantStatus: 200,
			write: func(w http.ResponseWriter) { _, _ = w.Write([]byte("ok")) }},
		{name: "a body after WriteHeader does not reset the status", wantStatus: 503,
			write: func(w http.ResponseWriter) { w.WriteHeader(503); _, _ = w.Write([]byte("nope")) }},
		// net/http ignores a second WriteHeader, so the client saw the first.
		{name: "only the first WriteHeader counts", wantStatus: 301,
			write: func(w http.ResponseWriter) { w.WriteHeader(301); w.WriteHeader(500) }},
		{name: "a handler that writes nothing is the implicit 200", wantStatus: 200,
			write: func(http.ResponseWriter) {}},
		// Nothing reached the client, so the initial 200 would be a fiction:
		// Recoverer is about to send a 500.
		{name: "a panic before anything is written is a 500", wantStatus: 500, recovered: panicked,
			write: func(http.ResponseWriter) {}},
		// The header is already on the wire and Recoverer cannot take it
		// back, so the status the client saw is the one to report.
		{name: "a panic after the header keeps the status the client saw", wantStatus: 201, recovered: panicked,
			write: func(w http.ResponseWriter) { w.WriteHeader(201) }},
		{name: "a panic after a bare Write keeps the implicit 200", wantStatus: 200, recovered: panicked,
			write: func(w http.ResponseWriter) { _, _ = w.Write([]byte("partial")) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			sw := NewStatusWriter(rec)
			tt.write(sw)

			if got := sw.ObservedStatus(tt.recovered); got != tt.wantStatus {
				t.Errorf("ObservedStatus(%v) = %d, want %d", tt.recovered, got, tt.wantStatus)
			}
			// Whatever the bookkeeping concludes, the bytes still have to
			// reach the underlying writer untouched.
			if tt.recovered == nil && rec.Code != sw.ObservedStatus(nil) {
				t.Errorf("underlying recorder status = %d, want %d", rec.Code, sw.ObservedStatus(nil))
			}
		})
	}
}

func TestStatusWriterPassesWritesThrough(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := NewStatusWriter(rec)

	sw.Header().Set("X-Test", "1")
	sw.WriteHeader(http.StatusAccepted)
	n, err := sw.Write([]byte("hello"))

	if err != nil || n != 5 {
		t.Errorf("Write = %d, %v, want 5, nil", n, err)
	}
	if rec.Code != http.StatusAccepted || rec.Body.String() != "hello" {
		t.Errorf("recorder = %d, %q, want 202, %q", rec.Code, rec.Body.String(), "hello")
	}
	if got := rec.Header().Get("X-Test"); got != "1" {
		t.Errorf("X-Test = %q, want it to reach the underlying writer", got)
	}
}

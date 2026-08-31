package worker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// capturedRequest is what the stub worker saw. traceHeaders separates "header
// absent" from "header present but empty", which Header.Get cannot express.
type capturedRequest struct {
	method, path, contentType, body, traceID string
	traceHeaders                             int
}

// newWorkerStub records the request it receives and answers with status/body.
func newWorkerStub(t *testing.T, status int, respBody string) (*httptest.Server, *capturedRequest) {
	t.Helper()

	got := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqBody, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		*got = capturedRequest{
			method: r.Method, path: r.URL.Path, contentType: r.Header.Get("Content-Type"),
			body: string(reqBody), traceID: r.Header.Get("X-Trace-Id"),
			traceHeaders: len(r.Header.Values("X-Trace-Id")),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(respBody))
	}))
	t.Cleanup(srv.Close)

	return srv, got
}

func TestAnalyzeURL(t *testing.T) {
	tests := []struct {
		name, baseURL, want string
	}{
		{name: "base url without a trailing slash", baseURL: "http://worker:8001", want: "http://worker:8001/analyze"},
		{name: "a trailing slash does not double up", baseURL: "http://worker:8001/", want: "http://worker:8001/analyze"},
		{name: "every trailing slash is trimmed", baseURL: "http://worker:8001///", want: "http://worker:8001/analyze"},
		{name: "a path prefix in the base url is kept", baseURL: "http://worker:8001/api", want: "http://worker:8001/api/analyze"},
		// A relative url, which http.Client rejects at send time. config.Load
		// never produces one, so this pins the behaviour rather than endorses it.
		{name: "an empty base url yields a relative path", baseURL: "", want: "/analyze"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzeURL(tt.baseURL); got != tt.want {
				t.Errorf("analyzeURL(%q) = %q, want %q", tt.baseURL, got, tt.want)
			}
		})
	}
}

func TestHTTPClientAnalyzeRequest(t *testing.T) {
	body := `{"type":"linear_regression","data":[1,2],"params":{}}`

	tests := []struct {
		name, suffix, traceID string
		wantTraceHeaders      int
	}{
		{name: "posts the body as json to /analyze and forwards the trace id", traceID: "abc-123", wantTraceHeaders: 1},
		{name: "sends no X-Trace-Id header when the trace id is empty", traceID: "", wantTraceHeaders: 0},
		// Both WORKER_BASE_URL spellings must land on /analyze, not //analyze.
		{name: "a base url with a trailing slash reaches the same /analyze", suffix: "/", traceID: "abc-123", wantTraceHeaders: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, got := newWorkerStub(t, http.StatusOK, `{"data":{}}`)
			c := &HTTPClient{BaseURL: srv.URL + tt.suffix, HTTP: srv.Client()}

			if _, _, err := c.Analyze(context.Background(), tt.traceID, []byte(body)); err != nil {
				t.Fatalf("Analyze: %v", err)
			}
			want := capturedRequest{
				method: http.MethodPost, path: "/analyze", contentType: "application/json",
				body: body, traceID: tt.traceID, traceHeaders: tt.wantTraceHeaders,
			}
			if *got != want {
				t.Errorf("request = %+v, want %+v", *got, want)
			}
		})
	}
}

func TestHTTPClientAnalyzePassesTheWorkerResponseThrough(t *testing.T) {
	// The worker already speaks the {data, error, meta} envelope, so its status
	// and bytes come back untouched -- failures are responses, not errors.
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "a 200 body is returned unchanged", status: http.StatusOK, body: `{"data":{"slope":1.5},"error":null}`},
		{name: "a 400 is not turned into an error", status: http.StatusBadRequest, body: `{"error":{"code":"invalid_params"}}`},
		{name: "a 500 is not turned into an error", status: http.StatusInternalServerError, body: `{"error":{"code":"internal_error"}}`},
		{name: "a body that is not json is passed through as-is", status: http.StatusBadGateway, body: "<html>bad gateway</html>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := newWorkerStub(t, tt.status, tt.body)
			c := &HTTPClient{BaseURL: srv.URL, HTTP: srv.Client()}

			status, respBody, err := c.Analyze(context.Background(), "trace-1", []byte(`{"type":"x"}`))
			if err != nil {
				t.Fatalf("Analyze: %v, want a nil error for a %d response", err, tt.status)
			}
			if status != tt.status || string(respBody) != tt.body {
				t.Errorf("status, body = %d, %q; want %d, %q", status, respBody, tt.status, tt.body)
			}
		})
	}
}

func TestHTTPClientAnalyzeFailures(t *testing.T) {
	// Every failure below reports status 0 and a nil body: the caller turns any
	// non-nil error into a 502 worker_unreachable and ignores the rest.
	assertNoResponse := func(t *testing.T, status int, respBody []byte, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("err = nil, want an error")
		}
		if status != 0 || respBody != nil {
			t.Errorf("status, body = %d, %q; want 0, nil", status, respBody)
		}
	}

	closed, _ := newWorkerStub(t, http.StatusOK, `{}`)
	closed.Close()
	// A control character makes url.Parse fail, so a malformed WORKER_BASE_URL
	// surfaces once per request instead of at startup, like an unreachable port.
	for _, tt := range []struct{ name, baseURL string }{
		{name: "a base url that cannot be parsed fails before anything is sent", baseURL: "http://worker:8001\x7f"},
		{name: "an unreachable worker returns an error", baseURL: closed.URL},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := &HTTPClient{BaseURL: tt.baseURL, HTTP: &http.Client{}}
			status, respBody, err := c.Analyze(context.Background(), "trace-1", []byte(`{}`))
			assertNoResponse(t, status, respBody, err)
		})
	}

	t.Run("an already cancelled context returns context.Canceled", func(t *testing.T) {
		srv, _ := newWorkerStub(t, http.StatusOK, `{}`)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		c := &HTTPClient{BaseURL: srv.URL, HTTP: srv.Client()}
		status, respBody, err := c.Analyze(ctx, "trace-1", []byte(`{}`))
		assertNoResponse(t, status, respBody, err)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want it to wrap context.Canceled", err)
		}
	})

	t.Run("a worker slower than the client timeout returns a deadline error", func(t *testing.T) {
		// The handler blocks until cleanup rather than sleeping, so the timeout
		// alone decides how long this takes.
		release := make(chan struct{})
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
		t.Cleanup(func() { close(release); srv.Close() })

		c := &HTTPClient{BaseURL: srv.URL, HTTP: &http.Client{Timeout: 50 * time.Millisecond}}
		status, respBody, err := c.Analyze(context.Background(), "trace-1", []byte(`{}`))
		assertNoResponse(t, status, respBody, err)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("err = %v, want it to wrap context.DeadlineExceeded", err)
		}
	})

	t.Run("a truncated response body discards the status code as well", func(t *testing.T) {
		// The status is known before the body is read, but a read failure drops
		// it and returns 0 -- so this 200 reaches the caller as a 502.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Length", "100")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":`))
		}))
		t.Cleanup(srv.Close)

		c := &HTTPClient{BaseURL: srv.URL, HTTP: srv.Client()}
		status, respBody, err := c.Analyze(context.Background(), "trace-1", []byte(`{}`))
		assertNoResponse(t, status, respBody, err)
	})
}

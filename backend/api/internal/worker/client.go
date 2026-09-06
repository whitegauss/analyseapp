// Package worker calls the Python analysis worker's HTTP API. The worker
// already responds with the same {data, error, meta} envelope the Go API
// uses (PDR.md section 8), so callers proxy the status code and body
// through verbatim rather than re-decoding them.
package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client sends an analysis request to the worker and returns its raw HTTP
// status code and JSON response body.
type Client interface {
	Analyze(ctx context.Context, traceID string, body []byte) (status int, respBody []byte, err error)
}

// HTTPClient is the real Client implementation, backed by net/http. Build it
// with NewHTTPClient: the fields are unexported so that a client which cannot
// work -- no HTTP client, or a base URL nothing can be sent to -- cannot be
// constructed from outside this package (KAN-64).
type HTTPClient struct {
	baseURL string
	http    *http.Client
}

// ErrNoHTTPClient is returned by Analyze when the http client is nil, rather
// than letting the nil dereference panic. NewHTTPClient rules that out, but
// the zero value is still reachable inside this package, and a panic here
// reaches the caller as middleware.Recoverer's bare 500, which says nothing
// about the cause; analyze.go maps this error to a 502 that names it.
var ErrNoHTTPClient = errors.New("worker: HTTP client not configured")

// NewHTTPClient builds a Client for the worker at baseURL, giving every
// request the same timeout -- which is the only bound on how long an analysis
// can occupy a handler, so there is no zero value to fall back to.
//
// baseURL is validated here rather than at send time. It comes from
// WORKER_BASE_URL, which has a default and so always lets the process start;
// a broken value used to surface once per request as a 502
// worker_unreachable, which reads as "the worker is down" and points an
// investigation at the wrong thing (KAN-62).
func NewHTTPClient(baseURL string, timeout time.Duration) (*HTTPClient, error) {
	if err := ValidateBaseURL(baseURL); err != nil {
		return nil, err
	}
	return &HTTPClient{baseURL: baseURL, http: &http.Client{Timeout: timeout}}, nil
}

// ValidateBaseURL reports whether raw is a URL the worker can actually be
// reached at. Exported so a caller can check a configured value and name the
// variable it came from in the failure.
func ValidateBaseURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("%w: it is empty", ErrInvalidBaseURL)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %q cannot be parsed: %w", ErrInvalidBaseURL, raw, err)
	}
	// A scheme net/http cannot send over fails inside Do, far from the
	// mistake; "worker:8001" parses cleanly as scheme "worker".
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: %q needs an http:// or https:// scheme", ErrInvalidBaseURL, raw)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: %q has no host", ErrInvalidBaseURL, raw)
	}
	return nil
}

// ErrInvalidBaseURL is returned by NewHTTPClient and ValidateBaseURL for a
// base URL no request could be sent to.
var ErrInvalidBaseURL = errors.New("worker: invalid base URL")

// analyzeURL joins the worker's base URL with the /analyze path. BaseURL comes
// from WORKER_BASE_URL and may or may not carry a trailing slash, so trailing
// slashes are trimmed to keep both spellings pointing at the same endpoint.
func analyzeURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/analyze"
}

func (c *HTTPClient) Analyze(ctx context.Context, traceID string, body []byte) (int, []byte, error) {
	if c.http == nil {
		return 0, nil, ErrNoHTTPClient
	}
	endpoint := analyzeURL(c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if traceID != "" {
		req.Header.Set("X-Trace-Id", traceID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, respBody, nil
}

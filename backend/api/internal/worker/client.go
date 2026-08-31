// Package worker calls the Python analysis worker's HTTP API. The worker
// already responds with the same {data, error, meta} envelope the Go API
// uses (PDR.md section 8), so callers proxy the status code and body
// through verbatim rather than re-decoding them.
package worker

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
)

// Client sends an analysis request to the worker and returns its raw HTTP
// status code and JSON response body.
type Client interface {
	Analyze(ctx context.Context, traceID string, body []byte) (status int, respBody []byte, err error)
}

// HTTPClient is the real Client implementation, backed by net/http.
type HTTPClient struct {
	BaseURL string
	HTTP    *http.Client
}

// analyzeURL joins the worker's base URL with the /analyze path. BaseURL comes
// from WORKER_BASE_URL and may or may not carry a trailing slash, so trailing
// slashes are trimmed to keep both spellings pointing at the same endpoint.
func analyzeURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/analyze"
}

func (c *HTTPClient) Analyze(ctx context.Context, traceID string, body []byte) (int, []byte, error) {
	url := analyzeURL(c.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if traceID != "" {
		req.Header.Set("X-Trace-Id", traceID)
	}

	resp, err := c.HTTP.Do(req)
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

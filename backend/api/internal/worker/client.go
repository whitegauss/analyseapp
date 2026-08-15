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

func (c *HTTPClient) Analyze(ctx context.Context, traceID string, body []byte) (int, []byte, error) {
	url := strings.TrimRight(c.BaseURL, "/") + "/analyze"
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

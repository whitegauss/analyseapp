package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"analyseapp/api/internal/cache"
	"analyseapp/api/internal/experiments"
)

// fakeWorkerClient is a minimal worker.Client implementation for handler
// tests, so handler logic is testable without a running Python worker.
type fakeWorkerClient struct {
	t         *testing.T
	analyzeFn func(ctx context.Context, traceID string, body []byte) (int, []byte, error)
}

func (f *fakeWorkerClient) Analyze(ctx context.Context, traceID string, body []byte) (int, []byte, error) {
	if f.analyzeFn == nil {
		f.t.Fatal("unexpected call to Analyze")
	}
	return f.analyzeFn(ctx, traceID, body)
}

// fakeCache is an in-memory cache.Cache implementation for handler tests.
type fakeCache struct {
	store map[string][]byte
}

func newFakeCache() *fakeCache {
	return &fakeCache{store: map[string][]byte{}}
}

func (c *fakeCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	v, ok := c.store[key]
	return v, ok, nil
}

func (c *fakeCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	c.store[key] = value
	return nil
}

func (c *fakeCache) DeleteByPrefix(ctx context.Context, prefix string) error {
	for key := range c.store {
		if strings.HasPrefix(key, prefix) {
			delete(c.store, key)
		}
	}
	return nil
}

func TestHandleAnalyzeExperiment(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		store := &fakeStore{t: t}
		wc := &fakeWorkerClient{t: t}
		req := newTestRequest("POST", uuid.New().String(), `{"type":"linear_regression"}`, false)
		rec := httptest.NewRecorder()

		handleAnalyzeExperiment(store, wc, newFakeCache())(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		store := &fakeStore{t: t}
		wc := &fakeWorkerClient{t: t}
		req := newTestRequest("POST", "not-a-uuid", `{"type":"linear_regression"}`, true)
		rec := httptest.NewRecorder()

		handleAnalyzeExperiment(store, wc, newFakeCache())(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_id" {
			t.Errorf("error code = %+v, want invalid_id", body.Error)
		}
	})

	t.Run("invalid body", func(t *testing.T) {
		store := &fakeStore{t: t}
		wc := &fakeWorkerClient{t: t}
		req := newTestRequest("POST", uuid.New().String(), `not json`, true)
		rec := httptest.NewRecorder()

		handleAnalyzeExperiment(store, wc, newFakeCache())(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_body" {
			t.Errorf("error code = %+v, want invalid_body", body.Error)
		}
	})

	t.Run("missing type", func(t *testing.T) {
		store := &fakeStore{t: t}
		wc := &fakeWorkerClient{t: t}
		req := newTestRequest("POST", uuid.New().String(), `{}`, true)
		rec := httptest.NewRecorder()

		handleAnalyzeExperiment(store, wc, newFakeCache())(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "invalid_type" {
			t.Errorf("error code = %+v, want invalid_type", body.Error)
		}
	})

	t.Run("experiment not found", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			getByIDFn: func(ctx context.Context, id, userID uuid.UUID) (experiments.Experiment, error) {
				return experiments.Experiment{}, experiments.ErrNotFound
			},
		}
		wc := &fakeWorkerClient{t: t}
		req := newTestRequest("POST", uuid.New().String(), `{"type":"linear_regression"}`, true)
		rec := httptest.NewRecorder()

		handleAnalyzeExperiment(store, wc, newFakeCache())(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("worker unreachable", func(t *testing.T) {
		store := &fakeStore{
			t: t,
			getByIDFn: func(ctx context.Context, id, userID uuid.UUID) (experiments.Experiment, error) {
				return experiments.Experiment{ID: id, UserID: userID, RawData: map[string]any{"columns": map[string]any{}}}, nil
			},
		}
		wc := &fakeWorkerClient{
			t: t,
			analyzeFn: func(ctx context.Context, traceID string, body []byte) (int, []byte, error) {
				return 0, nil, context.DeadlineExceeded
			},
		}
		req := newTestRequest("POST", uuid.New().String(), `{"type":"linear_regression"}`, true)
		rec := httptest.NewRecorder()

		handleAnalyzeExperiment(store, wc, newFakeCache())(rec, req)

		if rec.Code != http.StatusBadGateway {
			t.Errorf("status = %d, want 502", rec.Code)
		}
		if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "worker_unreachable" {
			t.Errorf("error code = %+v, want worker_unreachable", body.Error)
		}
	})

	t.Run("success passes worker response through verbatim and caches it", func(t *testing.T) {
		id := uuid.New()
		rawData := map[string]any{"columns": map[string]any{"x": []any{1.0, 2.0}, "y": []any{2.0, 4.0}}}
		store := &fakeStore{
			t: t,
			getByIDFn: func(ctx context.Context, gotID, userID uuid.UUID) (experiments.Experiment, error) {
				return experiments.Experiment{ID: gotID, UserID: userID, RawData: rawData}, nil
			},
		}
		workerResp := `{"data":{"type":"linear_regression","result":{"slope":2}},"error":null,"meta":{}}`
		var gotBody []byte
		var gotTraceID string
		wc := &fakeWorkerClient{
			t: t,
			analyzeFn: func(ctx context.Context, traceID string, body []byte) (int, []byte, error) {
				gotBody = body
				gotTraceID = traceID
				return http.StatusOK, []byte(workerResp), nil
			},
		}
		fc := newFakeCache()
		params := map[string]any{"weighted": true}
		req := newTestRequest("POST", id.String(), `{"type":"linear_regression","params":{"weighted":true}}`, true)
		rec := httptest.NewRecorder()

		handleAnalyzeExperiment(store, wc, fc)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
		if rec.Header().Get("X-Cache") != "MISS" {
			t.Errorf("X-Cache = %q, want MISS", rec.Header().Get("X-Cache"))
		}
		if rec.Body.String() != workerResp {
			t.Errorf("body = %s, want worker response passed through verbatim", rec.Body.String())
		}
		if gotBody == nil {
			t.Fatal("worker client was not called")
		}
		_ = gotTraceID

		var sentToWorker struct {
			Type   string         `json:"type"`
			Data   map[string]any `json:"data"`
			Params map[string]any `json:"params"`
		}
		if err := json.Unmarshal(gotBody, &sentToWorker); err != nil {
			t.Fatalf("decode body sent to worker: %v", err)
		}
		if sentToWorker.Type != "linear_regression" {
			t.Errorf("type sent to worker = %q, want linear_regression", sentToWorker.Type)
		}
		if sentToWorker.Params["weighted"] != true {
			t.Errorf("params sent to worker = %+v, want {weighted: true}", sentToWorker.Params)
		}
		columns, ok := sentToWorker.Data["columns"].(map[string]any)
		if !ok || len(columns) != 2 {
			t.Errorf("data.columns sent to worker = %+v, want the experiment's raw_data.columns", sentToWorker.Data)
		}

		key, err := cache.AnalysisKey(id, "linear_regression", params)
		if err != nil {
			t.Fatalf("compute cache key: %v", err)
		}
		if cached, ok := fc.store[key]; !ok || string(cached) != workerResp {
			t.Errorf("cache[%s] = %s, %v, want the worker response to be cached", key, cached, ok)
		}
	})

	t.Run("cache hit skips the store and the worker", func(t *testing.T) {
		id := uuid.New()
		params := map[string]any{}
		key, err := cache.AnalysisKey(id, "linear_regression", params)
		if err != nil {
			t.Fatalf("compute cache key: %v", err)
		}
		cachedResp := `{"data":{"type":"linear_regression","result":{"slope":2}},"error":null,"meta":{}}`
		fc := newFakeCache()
		fc.store[key] = []byte(cachedResp)

		store := &fakeStore{t: t}     // getByIDFn unset: t.Fatal if called
		wc := &fakeWorkerClient{t: t} // analyzeFn unset: t.Fatal if called
		req := newTestRequest("POST", id.String(), `{"type":"linear_regression"}`, true)
		rec := httptest.NewRecorder()

		handleAnalyzeExperiment(store, wc, fc)(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
		if rec.Header().Get("X-Cache") != "HIT" {
			t.Errorf("X-Cache = %q, want HIT", rec.Header().Get("X-Cache"))
		}
		if rec.Body.String() != cachedResp {
			t.Errorf("body = %s, want the cached response", rec.Body.String())
		}
	})
}

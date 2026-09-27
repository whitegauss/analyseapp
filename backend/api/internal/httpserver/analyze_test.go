package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"analyseapp/api/internal/cache"
	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/logging"
	"analyseapp/api/internal/response"
	"analyseapp/api/internal/worker"
)

// analyzeDeps is everything handleAnalyzeExperiment talks to. traced wraps
// the handler in logging.Middleware, the one place a trace ID comes from.
type analyzeDeps struct {
	store  *fakeStore
	worker *fakeWorkerClient
	cache  *fakeCache
	traced bool
}

func TestHandleAnalyzeExperiment(t *testing.T) {
	id := uuid.New()
	rawData := map[string]any{"columns": map[string]any{"x": []any{1.0, 2.0}, "y": []any{2.0, 4.0}}}
	params := map[string]any{"weighted": true}
	const reqBody = `{"type":"linear_regression","params":{"weighted":true}}`
	const workerOK = `{"data":{"type":"linear_regression","result":{"slope":2}},"error":null,"meta":{}}`
	const workerRejects = `{"data":null,"error":{"code":"insufficient_data","message":"at least 2 data points are required"},"meta":{}}`
	const workerFails = `{"data":null,"error":{"code":"internal_error","message":"boom"},"meta":{}}`
	key, err := cache.AnalysisKey(id, "linear_regression", params)
	if err != nil {
		t.Fatalf("compute cache key: %v", err)
	}

	// found answers GetByID with the experiment, as the store would for its
	// owner.
	found := func(t *testing.T) *fakeStore {
		return &fakeStore{t: t, getByIDFn: func(_ context.Context, gotID, userID uuid.UUID) (experiments.Experiment, error) {
			wantStoreCall(t, gotID, id, userID)
			return experiments.Experiment{ID: gotID, UserID: userID, RawData: rawData}, nil
		}}
	}
	// answering is a worker that replies status/body, or fails with err.
	answering := func(t *testing.T, status int, body string, err error) *fakeWorkerClient {
		return &fakeWorkerClient{t: t, analyzeFn: func(context.Context, string, []byte) (int, []byte, error) {
			return status, []byte(body), err
		}}
	}
	deps := func(store func(*testing.T) *fakeStore, worker func(*testing.T) *fakeWorkerClient, c func() *fakeCache) func(t *testing.T) analyzeDeps {
		return func(t *testing.T) analyzeDeps {
			d := analyzeDeps{store: &fakeStore{t: t}, worker: &fakeWorkerClient{t: t}, cache: newFakeCache()}
			if store != nil {
				d.store = store(t)
			}
			if worker != nil {
				d.worker = worker(t)
			}
			if c != nil {
				d.cache = c()
			}
			return d
		}
	}
	workerSays := func(status int, body string) func(*testing.T) *fakeWorkerClient {
		return func(t *testing.T) *fakeWorkerClient { return answering(t, status, body, nil) }
	}

	// The cache the rows share state through: each row's deps build a fresh
	// one, and its checks read the one its handler was given. Rows run in
	// order, never in parallel.
	var last analyzeDeps
	handler := func(d analyzeDeps) http.HandlerFunc {
		last = d
		h := handleAnalyzeExperiment(d.store, d.worker, d.cache)
		if d.traced {
			return logging.Middleware(h).ServeHTTP
		}
		return h
	}
	wantCached := func(want bool) func(t *testing.T, _ response.Envelope) {
		return func(t *testing.T, _ response.Envelope) {
			if _, ok := last.cache.store[key]; ok != want {
				t.Errorf("result cached = %v, want %v", ok, want)
			}
		}
	}
	// passedThrough checks the body is the worker's, byte for byte, and
	// says where it came from.
	passedThrough := func(body, xCache string) func(t *testing.T, rec *httptest.ResponseRecorder) {
		return func(t *testing.T, rec *httptest.ResponseRecorder) {
			if rec.Body.String() != body {
				t.Errorf("body = %s, want %s verbatim", rec.Body.String(), body)
			}
			if got := rec.Header().Get("X-Cache"); got != xCache {
				t.Errorf("X-Cache = %q, want %q", got, xCache)
			}
		}
	}

	// Set by the KAN-63 row's worker; read by its check.
	var logged bytes.Buffer
	var sentToWorker []byte
	var traceSent string

	runHandlerCases(t, "POST", []handlerCase[analyzeDeps]{
		{name: "unauthenticated", id: id.String(), body: reqBody, unauthenticated: true,
			wantStatus: http.StatusUnauthorized, wantCode: "unauthorized"},
		{name: "invalid id", id: "not-a-uuid", body: reqBody,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_id"},
		{name: "invalid body", id: id.String(), body: `not json`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_body"},
		{name: "missing type", id: id.String(), body: `{}`,
			wantStatus: http.StatusBadRequest, wantCode: "invalid_type"},
		{name: "experiment not found", id: id.String(), body: reqBody,
			store: deps(func(t *testing.T) *fakeStore {
				return &fakeStore{t: t, getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (experiments.Experiment, error) {
					return experiments.Experiment{}, experiments.ErrNotFound
				}}
			}, nil, nil),
			wantStatus: http.StatusNotFound, wantCode: "not_found"},

		{name: "success passes the worker response through verbatim and caches it", id: id.String(), body: reqBody,
			store: deps(found, func(t *testing.T) *fakeWorkerClient {
				return &fakeWorkerClient{t: t, analyzeFn: func(_ context.Context, _ string, body []byte) (int, []byte, error) {
					sentToWorker = body
					return http.StatusOK, []byte(workerOK), nil
				}}
			}, nil),
			wantStatus:    http.StatusOK,
			checkResponse: passedThrough(workerOK, "MISS"),
			check: func(t *testing.T, body response.Envelope) {
				wantCached(true)(t, body)
				var sent struct {
					Type   string         `json:"type"`
					Data   map[string]any `json:"data"`
					Params map[string]any `json:"params"`
				}
				if err := json.Unmarshal(sentToWorker, &sent); err != nil {
					t.Fatalf("decode body sent to worker: %v", err)
				}
				if sent.Type != "linear_regression" || sent.Params["weighted"] != true {
					t.Errorf("type, params sent to worker = %q, %+v, want the request's", sent.Type, sent.Params)
				}
				if columns, ok := sent.Data["columns"].(map[string]any); !ok || len(columns) != 2 {
					t.Errorf("data sent to worker = %+v, want the experiment's raw_data", sent.Data)
				}
			}},
		// The worker's own errors are answers, not outages: its status and
		// body reach the client unchanged (the envelopes match), and --
		// since only a 200 is cached -- a rejected or failed run is tried
		// again next time rather than served from the cache for a day.
		{name: "a worker 400 is passed through and not cached", id: id.String(), body: reqBody,
			store:      deps(found, workerSays(http.StatusBadRequest, workerRejects), nil),
			wantStatus: http.StatusBadRequest, wantCode: "insufficient_data",
			checkResponse: passedThrough(workerRejects, "MISS"), check: wantCached(false)},
		{name: "a worker 500 is passed through and not cached", id: id.String(), body: reqBody,
			store:      deps(found, workerSays(http.StatusInternalServerError, workerFails), nil),
			wantStatus: http.StatusInternalServerError, wantCode: "internal_error",
			checkResponse: passedThrough(workerFails, "MISS"), check: wantCached(false)},
		{name: "worker unreachable", id: id.String(), body: reqBody,
			store: deps(found, func(t *testing.T) *fakeWorkerClient {
				return answering(t, 0, "", context.DeadlineExceeded)
			}, nil),
			wantStatus: http.StatusBadGateway, wantCode: "worker_unreachable", check: wantCached(false)},
		// Every worker failure answers with the same 502, so the log is the
		// only place the reason survives -- and it used to be dropped here
		// entirely, leaving that 502 to read as "the worker is down" even
		// when the worker had answered and only the response was cut short
		// (KAN-63).
		{name: "the reason the worker call failed reaches the log", id: id.String(), body: reqBody,
			store: deps(found, func(t *testing.T) *fakeWorkerClient {
				logged.Reset()
				saved := log.Logger
				log.Logger = zerolog.New(&logged)
				t.Cleanup(func() { log.Logger = saved })
				return answering(t, 0, "", fmt.Errorf("%w (the worker had answered 200): unexpected EOF", worker.ErrReadResponse))
			}, nil),
			// The response to the client is deliberately unchanged.
			wantStatus: http.StatusBadGateway, wantCode: "worker_unreachable",
			check: func(t *testing.T, _ response.Envelope) {
				for _, want := range []string{"reading the response failed", "200", `"trace_id"`} {
					if !strings.Contains(logged.String(), want) {
						t.Errorf("log = %q, want it to contain %q", logged.String(), want)
					}
				}
			}},
		// The worker log and the API log have to share an ID for a failed
		// analysis to be traceable across the two.
		{name: "the request's trace ID reaches the worker", id: id.String(), body: reqBody,
			store: func(t *testing.T) analyzeDeps {
				d := deps(found, func(t *testing.T) *fakeWorkerClient {
					return &fakeWorkerClient{t: t, analyzeFn: func(_ context.Context, traceID string, _ []byte) (int, []byte, error) {
						traceSent = traceID
						return http.StatusOK, []byte(workerOK), nil
					}}
				}, nil)(t)
				d.traced = true
				return d
			},
			wantStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				if got := rec.Header().Get("X-Trace-Id"); traceSent == "" || traceSent != got {
					t.Errorf("trace ID sent to worker = %q, want the request's %q", traceSent, got)
				}
			}},

		{name: "cache hit skips the store and the worker", id: id.String(), body: reqBody,
			// Both fakes fail the test if called.
			store: deps(nil, nil, func() *fakeCache {
				c := newFakeCache()
				c.store[key] = []byte(workerOK)
				return c
			}),
			wantStatus: http.StatusOK, checkResponse: passedThrough(workerOK, "HIT")},
		// Caching is a performance optimization, not a correctness
		// dependency: an unreachable Redis costs a worker call, never the
		// result.
		{name: "a cache read failure is a miss", id: id.String(), body: reqBody,
			store: deps(found, workerSays(http.StatusOK, workerOK), func() *fakeCache {
				c := newFakeCache()
				c.getErr = errors.New("redis: connection refused")
				return c
			}),
			wantStatus: http.StatusOK, checkResponse: passedThrough(workerOK, "MISS")},
		{name: "a cache write failure still returns the result", id: id.String(), body: reqBody,
			store: deps(found, workerSays(http.StatusOK, workerOK), func() *fakeCache {
				c := newFakeCache()
				c.setErr = errors.New("redis: connection refused")
				return c
			}),
			wantStatus: http.StatusOK, checkResponse: passedThrough(workerOK, "MISS")},
	}, deps(nil, nil, nil), handler)
}

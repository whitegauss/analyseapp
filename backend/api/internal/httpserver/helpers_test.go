package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"analyseapp/api/internal/auth"
	"analyseapp/api/internal/response"
)

// Request and response helpers shared by every handler test in the package
// (KAN-38).

var testUserID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

// newTestRequest builds a request carrying a chi "id" URL param and,
// optionally, an authenticated user in context (mirroring what auth.Middleware
// would have set).
func newTestRequest(method, id, body string, authenticated bool) *http.Request {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, "/", strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, "/", nil)
	}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	if authenticated {
		ctx = auth.WithUserID(ctx, testUserID)
	}
	return r.WithContext(ctx)
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) response.Envelope {
	t.Helper()
	var body response.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v (body=%s)", err, rec.Body.String())
	}
	return body
}

// errStoreDown stands in for any store failure that is not a missing row
// (a dropped connection, a timeout): the handlers must answer 500 for it,
// never 404, and never leak it into the body.
var errStoreDown = errors.New("db is down")

// handlerCase is one row of a table-driven handler test: the request, the
// store fake the handler talks to, and the response it must produce. S is
// the fake's type (*fakeStore, *fakeProjectStore).
type handlerCase[S any] struct {
	name string
	// id is the chi "id" URL param; body is the raw request body.
	id, body string
	// unauthenticated sends the request with no user in context.
	unauthenticated bool
	// store builds the fake for this row. Left nil, the runner's default is
	// used, whose every method fails the test -- the right store for a
	// request that must be turned away before reaching it.
	store      func(t *testing.T) S
	wantStatus int
	// wantCode is the envelope's error.code. Empty means a success, which
	// is checked too: the envelope must then carry no error at all.
	wantCode string
	// check makes further assertions on the decoded envelope.
	check func(t *testing.T, body response.Envelope)
}

// runHandlerCases runs each row as a subtest: builds its store (or
// newStore's, when the row has none), sends method to the handler built
// from it, and checks the status and error code before the row's own check.
func runHandlerCases[S any](
	t *testing.T,
	method string,
	cases []handlerCase[S],
	newStore func(t *testing.T) S,
	handler func(S) http.HandlerFunc,
) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			build := newStore
			if tc.store != nil {
				build = tc.store
			}
			req := newTestRequest(method, tc.id, tc.body, !tc.unauthenticated)
			rec := httptest.NewRecorder()

			handler(build(t))(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body=%s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			body := decodeEnvelope(t, rec)
			switch {
			case tc.wantCode == "" && body.Error != nil:
				t.Errorf("error = %+v, want none", body.Error)
			case tc.wantCode != "" && (body.Error == nil || body.Error.Code != tc.wantCode):
				t.Errorf("error = %+v, want code %q", body.Error, tc.wantCode)
			}
			if tc.check != nil {
				tc.check(t, body)
			}
		})
	}
}

// wantStoreCall checks that a handler asked the store about the experiment
// in the URL on behalf of the authenticated user. The userID is the
// ownership filter's input: a handler passing anything else would read or
// write another user's rows.
func wantStoreCall(t *testing.T, gotID, wantID, gotUserID uuid.UUID) {
	t.Helper()
	if gotID != wantID {
		t.Errorf("id passed to store = %v, want %v", gotID, wantID)
	}
	if gotUserID != testUserID {
		t.Errorf("userID passed to store = %v, want the authenticated user %v", gotUserID, testUserID)
	}
}

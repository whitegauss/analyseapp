package httpserver

import (
	"analyseapp/api/internal/auth"
	"analyseapp/api/internal/response"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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

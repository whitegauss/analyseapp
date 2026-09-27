//go:build integration

package httpserver

import (
	"net/http/httptest"
	"testing"

	"analyseapp/api/internal/pgtest"
)

// The ready path needs a database that answers, so it runs with the
// repository tests against TEST_DATABASE_URL (KAN-31).
func TestHandleReadyz_DatabaseReachable(t *testing.T) {
	rec := httptest.NewRecorder()
	handleReadyz(pgtest.Pool(t))(rec, httptest.NewRequest("GET", "/readyz", nil))

	if rec.Code != 200 {
		t.Errorf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	body := decodeEnvelope(t, rec)
	if data, _ := body.Data.(map[string]any); body.Error != nil || data["status"] != "ok" {
		t.Errorf("body = %+v, want {status: ok} and no error", body)
	}
}

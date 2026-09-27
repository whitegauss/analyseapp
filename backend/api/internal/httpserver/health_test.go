package httpserver

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"analyseapp/api/internal/response"
)

func TestHandleHealthz(t *testing.T) {
	req := httptest.NewRequest("GET", "/healthz", nil)
	rec := httptest.NewRecorder()

	handleHealthz(rec, req)

	if rec.Code != 200 {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var body response.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error != nil {
		t.Errorf("Error = %+v, want nil", body.Error)
	}
	if data, _ := body.Data.(map[string]any); data["status"] != "ok" {
		t.Errorf("Data = %+v, want {status: ok}", body.Data)
	}
}

func TestHandleReadyz_NoDatabase(t *testing.T) {
	req := httptest.NewRequest("GET", "/readyz", nil)
	rec := httptest.NewRecorder()

	handleReadyz(nil)(rec, req)

	if rec.Code != 503 {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	var body response.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error == nil || body.Error.Code != "db_not_configured" {
		t.Errorf("Error = %+v, want code db_not_configured", body.Error)
	}
}

// A configured database that cannot be reached is the case readiness exists
// for: the process is up (healthz says so) but cannot serve anything that
// touches the database. No database is needed to test it -- a pool pointed
// at a port nothing listens on connects lazily, so only the ping fails.
func TestHandleReadyz_DatabaseUnreachable(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://nobody@127.0.0.1:1/none?connect_timeout=1")
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	t.Cleanup(pool.Close)

	rec := httptest.NewRecorder()
	handleReadyz(pool)(rec, httptest.NewRequest("GET", "/readyz", nil))

	if rec.Code != 503 {
		t.Errorf("status = %d, want 503", rec.Code)
	}
	if body := decodeEnvelope(t, rec); body.Error == nil || body.Error.Code != "db_unreachable" {
		t.Errorf("Error = %+v, want code db_unreachable", body.Error)
	}
}

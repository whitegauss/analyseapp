package httpserver

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

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

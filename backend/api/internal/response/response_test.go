package response

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteData(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteData(rec, 201, map[string]string{"id": "abc"})

	if rec.Code != 201 {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error != nil {
		t.Errorf("Error = %+v, want nil", body.Error)
	}
	data, ok := body.Data.(map[string]any)
	if !ok || data["id"] != "abc" {
		t.Errorf("Data = %+v, want {id: abc}", body.Data)
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, 404, "not_found", "experiment not found")

	if rec.Code != 404 {
		t.Errorf("status = %d, want 404", rec.Code)
	}

	var body Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data != nil {
		t.Errorf("Data = %+v, want nil", body.Data)
	}
	if body.Error == nil {
		t.Fatal("Error = nil, want non-nil")
	}
	if body.Error.Code != "not_found" || body.Error.Message != "experiment not found" {
		t.Errorf("Error = %+v, want {not_found, experiment not found}", body.Error)
	}
}

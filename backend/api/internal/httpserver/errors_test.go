package httpserver

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/projects"
)

func TestStoreErrorResponse(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		res         storeResource
		failedTo    string
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{name: "experiment not found", err: experiments.ErrNotFound, res: experimentResource,
			failedTo: "get experiment", wantStatus: 404, wantCode: "not_found", wantMessage: "experiment not found"},
		{name: "project not found", err: projects.ErrNotFound, res: projectResource,
			failedTo: "get project", wantStatus: 404, wantCode: "not_found", wantMessage: "project not found"},
		{name: "an unrecognized error is a server fault", err: errors.New("boom"), res: experimentResource,
			failedTo: "delete experiment", wantStatus: 500, wantCode: "internal_error", wantMessage: "failed to delete experiment"},
		{name: "failedTo is echoed verbatim", err: errors.New("boom"), res: projectResource,
			failedTo: "update project", wantStatus: 500, wantCode: "internal_error", wantMessage: "failed to update project"},
		// The two sentinels are distinct values, so each resource recognizes
		// only its own: a projects error reaching an experiment handler is
		// reported as a server fault, not as a 404.
		{name: "the other resource's sentinel is not recognized", err: projects.ErrNotFound, res: experimentResource,
			failedTo: "get experiment", wantStatus: 500, wantCode: "internal_error", wantMessage: "failed to get experiment"},
		// Sentinels are compared with ==, so a wrapped ErrNotFound is a 500
		// even though errors.Is would match it. No store wraps it today;
		// pinned so that switching to errors.Is stays a deliberate change.
		{name: "a wrapped sentinel is not recognized", err: fmt.Errorf("query: %w", experiments.ErrNotFound),
			res: experimentResource, failedTo: "get experiment", wantStatus: 500, wantCode: "internal_error", wantMessage: "failed to get experiment"},
		// Callers only reach this function with a non-nil error; falling
		// through to 500 is the safe direction, so pin it.
		{name: "nil is not a not-found", err: nil, res: experimentResource,
			failedTo: "list experiments", wantStatus: 500, wantCode: "internal_error", wantMessage: "failed to list experiments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, message := storeErrorResponse(tt.err, tt.res, tt.failedTo)
			if status != tt.wantStatus || code != tt.wantCode || message != tt.wantMessage {
				t.Errorf("storeErrorResponse(%v, %q, %q) = (%d, %q, %q), want (%d, %q, %q)",
					tt.err, tt.res.name, tt.failedTo, status, code, message,
					tt.wantStatus, tt.wantCode, tt.wantMessage)
			}
		})
	}
}

// The write* adapters only have to put storeErrorResponse's answer into the
// envelope, so one case each is enough; the mapping itself is covered above.
func TestWriteStoreErrors(t *testing.T) {
	check := func(t *testing.T, rec *httptest.ResponseRecorder, status int, code, message string) {
		t.Helper()
		if rec.Code != status {
			t.Errorf("status = %d, want %d", rec.Code, status)
		}
		body := decodeEnvelope(t, rec)
		if body.Error == nil {
			t.Fatalf("error is nil, want %q", code)
		}
		if body.Error.Code != code || body.Error.Message != message {
			t.Errorf("error = (%q, %q), want (%q, %q)", body.Error.Code, body.Error.Message, code, message)
		}
		if body.Data != nil {
			t.Errorf("data = %v, want nil on an error response", body.Data)
		}
	}

	t.Run("writeExperimentError", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeExperimentError(rec, experiments.ErrNotFound, "get experiment")
		check(t, rec, 404, "not_found", "experiment not found")
	})

	t.Run("writeProjectError", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeProjectError(rec, errors.New("boom"), "update project")
		check(t, rec, 500, "internal_error", "failed to update project")
	})
}

// Package response provides the unified API response envelope
// (PDR.md section 8: {data, error, meta}), shared by httpserver and auth so
// neither has to import the other.
package response

import (
	"encoding/json"
	"net/http"
)

// Envelope is the unified response shape described in PDR.md section 8:
// {data, error, meta}.
type Envelope struct {
	Data  any       `json:"data"`
	Error *APIError `json:"error"`
	Meta  any       `json:"meta"`
}

// APIError represents a machine-readable error payload.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteData writes a successful envelope response with the given status code.
func WriteData(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, Envelope{Data: data, Error: nil, Meta: map[string]any{}})
}

// WriteError writes a failed envelope response with the given status code.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, Envelope{Data: nil, Error: &APIError{Code: code, Message: message}, Meta: map[string]any{}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

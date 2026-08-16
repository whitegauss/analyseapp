package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"analyseapp/api/internal/auth"
	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/response"
)

// requireUserID extracts the authenticated user from the request context,
// writing a 401 response and returning ok=false if there isn't one.
func requireUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, ok := auth.UserID(r.Context())
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authenticated user")
		return uuid.UUID{}, false
	}
	return userID, true
}

// parseIDParam parses the chi "id" URL param as a UUID, writing a 400
// response and returning ok=false if it's malformed.
func parseIDParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid_id", "id is not a valid UUID")
		return uuid.UUID{}, false
	}
	return id, true
}

// decodeJSONBody decodes r.Body into dst, writing a 400 response and
// returning false if it's malformed.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid_body", "could not parse JSON body")
		return false
	}
	return true
}

// writeExperimentError maps a experiments.Store error to the right HTTP
// response: 404 for ErrNotFound (which never distinguishes "doesn't exist"
// from "not yours", so ownership isn't leaked), 500 otherwise. failedTo
// completes the message "failed to <failedTo>" (e.g. "get experiment").
func writeExperimentError(w http.ResponseWriter, err error, failedTo string) {
	if err == experiments.ErrNotFound {
		response.WriteError(w, http.StatusNotFound, "not_found", "experiment not found")
		return
	}
	response.WriteError(w, http.StatusInternalServerError, "internal_error", "failed to "+failedTo)
}

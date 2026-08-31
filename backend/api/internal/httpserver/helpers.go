package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"analyseapp/api/internal/auth"
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
// response and returning ok=false if it's malformed. See parseID for what
// counts as malformed.
func parseIDParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid_id", "id is not a valid UUID")
		return uuid.UUID{}, false
	}
	return id, true
}

// decodeJSONBody decodes r.Body into dst, writing a 400 response and
// returning false if it's malformed. See decodeJSON for what counts as
// malformed.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := decodeJSON(r.Body, dst); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid_body", "could not parse JSON body")
		return false
	}
	return true
}

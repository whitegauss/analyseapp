package httpserver

import (
	"errors"
	"net/http"

	"analyseapp/api/internal/experiments"
	"analyseapp/api/internal/projects"
	"analyseapp/api/internal/response"
)

// storeResource pairs the name a resource goes by in error messages with the
// sentinel its store returns for a missing row, so a call site cannot pair
// one resource's name with another's sentinel.
type storeResource struct {
	name     string
	notFound error
}

var (
	experimentResource = storeResource{name: "experiment", notFound: experiments.ErrNotFound}
	projectResource    = storeResource{name: "project", notFound: projects.ErrNotFound}
)

// storeErrorResponse maps a store error to the HTTP response it deserves and
// returns it as values, so the mapping is testable without a
// http.ResponseWriter: 404 for the resource's ErrNotFound (which never
// distinguishes "doesn't exist" from "not yours", so ownership isn't
// leaked), 500 for anything else. failedTo completes the message "failed to
// <failedTo>" (e.g. "get experiment").
//
// The sentinel is matched with errors.Is, so a store is free to add context
// with %w. Compared with == instead, wrapping the sentinel -- the idiomatic
// thing to do -- would silently turn a 404 into a 500, and no handler test
// would catch it: the fakes return bare sentinels (KAN-66).
func storeErrorResponse(err error, res storeResource, failedTo string) (status int, code, message string) {
	if errors.Is(err, res.notFound) {
		return http.StatusNotFound, "not_found", res.name + " not found"
	}
	return http.StatusInternalServerError, "internal_error", "failed to " + failedTo
}

// writeExperimentError writes the response storeErrorResponse picked for an
// experiments.Store error.
func writeExperimentError(w http.ResponseWriter, err error, failedTo string) {
	status, code, message := storeErrorResponse(err, experimentResource, failedTo)
	response.WriteError(w, status, code, message)
}

// writeProjectError writes the response storeErrorResponse picked for a
// projects.Store error.
func writeProjectError(w http.ResponseWriter, err error, failedTo string) {
	status, code, message := storeErrorResponse(err, projectResource, failedTo)
	response.WriteError(w, status, code, message)
}

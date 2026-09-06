package response

import "net/http"

// StatusWriter wraps a ResponseWriter to remember what the client was
// actually sent. Both logging.Middleware and metrics.Middleware need exactly
// this, and the bookkeeping is subtle enough (see ObservedStatus) that two
// copies would eventually disagree about what a request's status was --
// which is the sort of divergence nobody notices until the logs and the
// metrics tell different stories about the same outage.
type StatusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// NewStatusWriter wraps w. The initial status is 200 because that is what a
// handler which only calls Write sends implicitly.
func NewStatusWriter(w http.ResponseWriter) *StatusWriter {
	return &StatusWriter{ResponseWriter: w, status: http.StatusOK}
}

func (sw *StatusWriter) WriteHeader(status int) {
	if !sw.wroteHeader {
		sw.status = status
		sw.wroteHeader = true
	}
	sw.ResponseWriter.WriteHeader(status)
}

func (sw *StatusWriter) Write(b []byte) (int, error) {
	sw.wroteHeader = true
	return sw.ResponseWriter.Write(b)
}

// ObservedStatus returns the status the client actually saw. recovered is the
// value from recover(), or nil if the handler returned normally.
//
// A handler that panicked before writing anything sent nothing, so the
// initial 200 would be a fiction: chi's middleware.Recoverer is about to send
// a 500 and that is what the client gets. Once a header is on the wire
// Recoverer cannot take it back, so a panic after that point keeps the status
// already sent -- an alert on 5xx will not see that request, which is why
// http_panics_total exists alongside it.
func (sw *StatusWriter) ObservedStatus(recovered any) int {
	if recovered != nil && !sw.wroteHeader {
		return http.StatusInternalServerError
	}
	return sw.status
}

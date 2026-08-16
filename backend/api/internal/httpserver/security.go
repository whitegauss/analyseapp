package httpserver

import "net/http"

// securityHeaders sets baseline hardening headers on every response. This
// service only ever serves the JSON envelope (never HTML), so there is
// deliberately no Content-Security-Policy here -- one written for an HTML
// document wouldn't mean anything applied to a JSON API response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// Ignored by browsers on a plain-HTTP connection (per the HSTS spec),
		// so this is harmless in local dev and takes effect automatically
		// once a production deployment terminates TLS in front of this
		// service.
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

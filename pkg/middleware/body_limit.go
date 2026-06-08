package middleware

import (
	"net/http"
)

// NewBodySizeLimit caps how many bytes a handler will read from the request
// body, protecting against memory exhaustion from oversized or malicious
// payloads. It wraps the body in an http.MaxBytesReader, which stops reading
// after maxBytes and causes reads to fail (the server responds 413 Request
// Entity Too Large) once the limit is exceeded.
//
// A maxBytes value of zero or less disables the limit and the next handler is
// returned unwrapped.
func NewBodySizeLimit(maxBytes int64, next http.Handler) http.Handler {
	if maxBytes <= 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
		next.ServeHTTP(w, r)
	})
}

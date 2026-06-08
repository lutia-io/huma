package middleware

import (
	"net/http"
	"time"
)

// NewTimeout enforces a per-request wall-clock deadline. If the downstream
// handler does not finish within timeout, http.TimeoutHandler responds with
// 503 Service Unavailable and the given body, and the handler's request
// context is canceled so cooperative work can stop.
//
// Note that the timed-out response body is served as text/plain by the standard
// library even though we send JSON; clients should rely on the status code.
//
// A timeout of zero or less disables the limit and the next handler is returned
// unwrapped.
func NewTimeout(timeout time.Duration, next http.Handler) http.Handler {
	if timeout <= 0 {
		return next
	}
	return http.TimeoutHandler(next, timeout, `{"error":"Request timed out"}`)
}

package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type requestIDKey struct{}

// RequestIDHeader is the canonical header used to carry the request ID on both
// the inbound request and the outbound response.
const RequestIDHeader = "X-Request-Id"

// maxRequestIDLen bounds the length of a client-supplied request ID. Without a
// limit a caller could send an arbitrarily large header that we then echo back
// and write into every log line for the request.
const maxRequestIDLen = 128

// ContextWithRequestID returns a copy of ctx that carries id. It is the single
// writer of the request-ID context value, used by NewRequestID and available
// to callers (e.g. tests, background jobs) that need to propagate an ID.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext returns the request ID stored by NewRequestID, if any.
// The boolean is false when no (non-empty) ID is present.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey{}).(string)
	if !ok {
		return "", false
	}
	if !isValidRequestID(id) {
		return "", false
	}
	return id, true
}

// NewRequestID ensures every request has a stable, safe request ID.
//
// If the client supplies one via RequestIDHeader and it passes validation we
// reuse it so the ID can be correlated across services. Otherwise (or if the
// supplied value is unsafe) we generate a fresh random ID. The chosen ID is
// echoed back on the response header and stored in the request context for
// downstream handlers and other middleware (e.g. logging and panic recovery).
func NewRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if !isValidRequestID(id) {
			id = newRequestID()
		}

		w.Header().Set(RequestIDHeader, id)
		ctx := ContextWithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isValidRequestID reports whether a client-supplied ID is safe to echo back
// and log. It rejects empty/oversized values and anything outside a
// conservative ASCII set so the value cannot be used for header or log
// injection.
func isValidRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '-' || c == '_' || c == '.':
		default:
			return false
		}
	}
	return true
}

// randRead is indirected through a variable so tests can simulate a failing
// entropy source.
var randRead = rand.Read

// newRequestID returns a random hex-encoded ID. crypto/rand essentially never
// fails, but if it does we fall back to a shorter ID rather than panicking.
func newRequestID() string {
	var b [16]byte
	_, err := randRead(b[:])
	if err != nil {
		var b2 [8]byte
		_, _ = randRead(b2[:])
		return hex.EncodeToString(b2[:])
	}
	return hex.EncodeToString(b[:])
}

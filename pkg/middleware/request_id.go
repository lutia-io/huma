package middleware

import (
	"context"
	"net/http"

	"github.com/lutia-io/huma/pkg/requestid"
)

// RequestIDHeader is the response header used to return the server-generated
// request ID to the client.
const RequestIDHeader = "X-Request-Id"

// NewRequestID generates a request ID for every request, stores it in the
// request context, and echoes it on the response header for downstream handlers
// and other middleware (e.g. logging and panic recovery).
func NewRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestid.New()
		w.Header().Set(RequestIDHeader, id)
		ctx := requestid.WithContext(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ContextWithRequestID returns a copy of ctx that carries id. It is the single
// writer of the request-ID context value, used by NewRequestID and available
// to callers (e.g. tests, background jobs) that need to propagate an ID.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return requestid.WithContext(ctx, id)
}

// RequestIDFromContext returns the request ID stored by NewRequestID, if any.
// The boolean is false when no ID is present.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	return requestid.FromContext(ctx)
}

package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/lutia-io/huma/pkg/render"
)

// NewRecover guards downstream handlers against panics so a single bad request
// cannot take down the server. When a panic is caught it logs the value, the
// request method/path, the stack trace and (when available) the request ID,
// then responds with a generic 500.
//
// http.ErrAbortHandler is intentionally re-panicked: the standard library uses
// it as a sentinel to silently abort a connection.
func NewRecover(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}

			attrs := []any{
				slog.Any("panic", rec),
				slog.String("path", r.URL.Path),
				slog.String("method", r.Method),
				slog.String("stack", string(debug.Stack())),
			}
			if rid, ok := RequestIDFromContext(r.Context()); ok {
				attrs = append(attrs, slog.String("request_id", rid))
			}
			logger.ErrorContext(r.Context(), "Panic recovered", attrs...)
			render.WriteInternalError(w)
		}()
		next.ServeHTTP(w, r)
	})
}

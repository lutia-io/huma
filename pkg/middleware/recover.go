package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/render"
)

// NewRecover guards downstream handlers against panics so a single bad request
// cannot take down the server. When a panic is caught it logs the value, the
// request method/path, the stack trace and (when available) the request ID,
// then responds with a generic 500.
//
// http.ErrAbortHandler is intentionally re-panicked: the standard library uses
// it as a sentinel to silently abort a connection.
func NewRecover(log *logger.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec)
			}

			log.ErrorContext(r.Context(), "Panic recovered",
				logger.KeyPanic, rec,
				logger.KeyPath, r.URL.Path,
				logger.KeyMethod, r.Method,
				logger.KeyStack, string(debug.Stack()),
			)
			render.WriteInternalError(w)
		}()
		next.ServeHTTP(w, r)
	})
}

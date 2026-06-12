package middleware

import (
	"net/http"
	"time"

	"github.com/lutia-io/huma/pkg/logger"
)

// responseRecorder wraps http.ResponseWriter to capture the status code and the
// number of body bytes written so the logger can report them after the handler
// returns. A zero status means WriteHeader has not been called yet, so the
// field doubles as the "header written" sentinel (0 is never a valid HTTP
// status code).
type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (rw *responseRecorder) WriteHeader(code int) {
	// Only the first WriteHeader call has any effect at the transport layer, so
	// mirror that here to record the status the client actually receives.
	if rw.status != 0 {
		return
	}
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseRecorder) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

// Unwrap exposes the underlying ResponseWriter so http.ResponseController can
// reach optional interfaces (http.Flusher, http.Hijacker, ...) that this
// wrapper does not implement directly.
func (rw *responseRecorder) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// NewLogger logs twice per request: once when the request arrives and again
// after the downstream handler finishes. The completion log includes the
// response status, body size and total duration, and its level reflects the
// outcome (5xx -> error, 4xx -> warn, otherwise info).
func NewLogger(log *logger.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		startAttrs := []any{
			logger.KeyMethod, r.Method,
			logger.KeyPath, r.URL.Path,
			logger.KeyRemoteAddr, r.RemoteAddr,
		}
		if q := r.URL.RawQuery; q != "" {
			startAttrs = append(startAttrs, logger.KeyQuery, q)
		}
		log.InfoContext(r.Context(), "http request started", startAttrs...)

		rw := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(rw, r)

		status := rw.status
		if status == 0 {
			status = http.StatusOK
		}

		attrs := []any{
			logger.KeyMethod, r.Method,
			logger.KeyPath, r.URL.Path,
			logger.KeyStatus, status,
			logger.KeyBytes, rw.bytes,
			logger.KeyDuration, time.Since(start),
			logger.KeyRemoteAddr, r.RemoteAddr,
		}
		if q := r.URL.RawQuery; q != "" {
			attrs = append(attrs, logger.KeyQuery, q)
		}

		switch {
		case status >= 500:
			log.ErrorContext(r.Context(), "http request completed", attrs...)
		case status >= 400:
			log.WarnContext(r.Context(), "http request completed", attrs...)
		default:
			log.InfoContext(r.Context(), "http request completed", attrs...)
		}
	})
}

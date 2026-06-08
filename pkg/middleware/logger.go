package middleware

import (
	"log/slog"
	"net/http"
	"time"
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
func NewLogger(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Extract the request ID from the context.
		rid, hasRID := RequestIDFromContext(r.Context())

		// Log the request arrival.
		startAttrs := []any{
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("remote_addr", r.RemoteAddr),
		}
		if q := r.URL.RawQuery; q != "" {
			startAttrs = append(startAttrs, slog.String("query", q))
		}
		if hasRID {
			startAttrs = append(startAttrs, slog.String("request_id", rid))
		}
		logger.InfoContext(r.Context(), "http request started", startAttrs...)

		// Wrap the response writer to capture the status code and the number of
		// body bytes written.
		rw := &responseRecorder{ResponseWriter: w}

		// Serve the request.
		next.ServeHTTP(rw, r)

		// A zero status means the handler returned without ever writing, in
		// which case net/http sends a 200 to the client; log what the client
		// actually receives.
		status := rw.status
		if status == 0 {
			status = http.StatusOK
		}

		// Log the request completion.
		attrs := []any{
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", status),
			slog.Int("bytes", rw.bytes),
			slog.Duration("duration", time.Since(start)),
			slog.String("remote_addr", r.RemoteAddr),
		}
		if q := r.URL.RawQuery; q != "" {
			attrs = append(attrs, slog.String("query", q))
		}
		if hasRID {
			attrs = append(attrs, slog.String("request_id", rid))
		}

		switch {
		case status >= 500:
			logger.ErrorContext(r.Context(), "http request completed", attrs...)
		case status >= 400:
			logger.WarnContext(r.Context(), "http request completed", attrs...)
		default:
			logger.InfoContext(r.Context(), "http request completed", attrs...)
		}
	})
}

package metrics

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "huma_http_requests_total",
		Help: "Total HTTP requests handled by the Huma API.",
	}, []string{"method", "path", "code"})

	duration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "huma_http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path", "code"})
)

// Handler exposes the default Prometheus registry at GET /metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rw *statusRecorder) WriteHeader(code int) {
	if rw.status != 0 {
		return
	}
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *statusRecorder) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

func (rw *statusRecorder) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// NewHTTP records request count and duration. Path labels use the ServeMux
// pattern when available so URL parameters do not explode cardinality.
func NewHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rw, r)

		status := rw.status
		if status == 0 {
			status = http.StatusOK
		}
		code := strconv.Itoa(status)
		path := routePath(r)
		requests.WithLabelValues(r.Method, path, code).Inc()
		duration.WithLabelValues(r.Method, path, code).Observe(time.Since(start).Seconds())
	})
}

func routePath(r *http.Request) string {
	if p := r.Pattern; p != "" {
		if _, rest, ok := strings.Cut(p, " "); ok && strings.HasPrefix(rest, "/") {
			return rest
		}
		if strings.HasPrefix(p, "/") {
			return p
		}
	}
	return r.URL.Path
}

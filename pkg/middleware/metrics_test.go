package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	dto "github.com/prometheus/client_model/go"
)

func TestNewMetrics_recordsRequest(t *testing.T) {
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello/{name}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	handler := NewMetrics(mux)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/hello/world", nil))

	count := counterValue(t, httpRequestsTotal.WithLabelValues(http.MethodGet, "GET /hello/{name}", "201"))
	if count != 1 {
		t.Fatalf("request count: got %v want 1", count)
	}
}

func TestNewMetrics_defaultsStatusOK(t *testing.T) {
	httpRequestsTotal.Reset()
	httpRequestDuration.Reset()

	handler := NewMetrics(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/healthz", nil))

	count := counterValue(t, httpRequestsTotal.WithLabelValues(http.MethodGet, "/healthz", "200"))
	if count != 1 {
		t.Fatalf("request count: got %v want 1", count)
	}
}

func counterValue(t *testing.T, metric interface {
	Write(*dto.Metric) error
}) float64 {
	t.Helper()

	var m dto.Metric
	if err := metric.Write(&m); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	return m.GetCounter().GetValue()
}

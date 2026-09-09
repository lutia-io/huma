package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewHTTP_recordsMuxPattern(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	h := NewHTTP(mux)
	req := httptest.NewRequest(http.MethodGet, "/users/abc", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusNoContent)
	}

	body := scrape(t)
	if !strings.Contains(body, `huma_http_requests_total{code="204",method="GET",path="/users/{id}"}`) {
		t.Fatalf("expected patterned path in metrics, got %q", body)
	}
	if strings.Contains(body, `/users/abc`) {
		t.Fatalf("raw URL must not appear as a path label, got %q", body)
	}
}

func TestNewHTTP_skipsMetricsPath(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := NewHTTP(inner)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := scrape(t)
	if strings.Contains(body, `path="/metrics"`) {
		t.Fatalf("scrapes of /metrics should not be recorded, got %q", body)
	}
}

func scrape(t *testing.T) string {
	t.Helper()

	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	b, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read metrics: %v", err)
	}
	return string(b)
}

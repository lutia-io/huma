package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lutia-io/huma/pkg/logger"
)

func testLogger(t *testing.T) (*logger.Logger, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	old := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(old) })

	return logger.New(), &buf
}

func TestResponseRecorder_writeHeaderOnce(t *testing.T) {
	inner := httptest.NewRecorder()
	rw := &responseRecorder{ResponseWriter: inner}

	rw.WriteHeader(http.StatusCreated)
	rw.WriteHeader(http.StatusBadRequest)

	if rw.status != http.StatusCreated {
		t.Fatalf("status: got %d want %d", rw.status, http.StatusCreated)
	}
	if inner.Code != http.StatusCreated {
		t.Fatalf("inner status: got %d want %d", inner.Code, http.StatusCreated)
	}
}

func TestResponseRecorder_writeSetsOK(t *testing.T) {
	inner := httptest.NewRecorder()
	rw := &responseRecorder{ResponseWriter: inner}

	n, err := rw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 5 {
		t.Fatalf("Write n: got %d want 5", n)
	}
	if rw.status != http.StatusOK {
		t.Fatalf("status: got %d want %d", rw.status, http.StatusOK)
	}
	if rw.bytes != 5 {
		t.Fatalf("bytes: got %d want 5", rw.bytes)
	}
}

func TestResponseRecorder_unwrap(t *testing.T) {
	inner := httptest.NewRecorder()
	rw := &responseRecorder{ResponseWriter: inner}

	if rw.Unwrap() != inner {
		t.Fatal("Unwrap: expected underlying ResponseWriter")
	}
}

func TestNewLogger_info(t *testing.T) {
	log, buf := testLogger(t)

	h := NewLogger(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rec.Code)
	}
	out := buf.String()
	if !strings.Contains(out, "http request started") || !strings.Contains(out, "http request completed") {
		t.Fatalf("logs: got %q", out)
	}
	if strings.Contains(out, `"level":"WARN"`) || strings.Contains(out, `"level":"ERROR"`) {
		t.Fatalf("expected info-level completion log, got %q", out)
	}
}

func TestNewLogger_noResponseDefaultsOK(t *testing.T) {
	log, buf := testLogger(t)

	h := NewLogger(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !strings.Contains(buf.String(), `"status":200`) {
		t.Fatalf("expected default status 200 in log, got %q", buf.String())
	}
}

func TestNewLogger_warn(t *testing.T) {
	log, buf := testLogger(t)

	h := NewLogger(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/missing", nil))

	if !strings.Contains(buf.String(), `"level":"WARN"`) {
		t.Fatalf("expected warn-level completion log, got %q", buf.String())
	}
}

func TestNewLogger_error(t *testing.T) {
	log, buf := testLogger(t)

	h := NewLogger(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/broken", nil))

	if !strings.Contains(buf.String(), `"level":"ERROR"`) {
		t.Fatalf("expected error-level completion log, got %q", buf.String())
	}
}

func TestNewLogger_withQuery(t *testing.T) {
	log, buf := testLogger(t)

	h := NewLogger(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/search?q=go", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	if !strings.Contains(out, `"query":"q=go"`) {
		t.Fatalf("expected query in logs, got %q", out)
	}
}

func TestNewLogger_responseControllerUnwrap(t *testing.T) {
	log, _ := testLogger(t)

	h := NewLogger(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = http.NewResponseController(w)
		w.WriteHeader(http.StatusOK)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

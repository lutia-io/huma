package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRecover_noPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	h := NewRecover(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rec.Code)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no logs on success, got: %q", buf.String())
	}
}

func TestNewRecover_recoversPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	h := NewRecover(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/explode", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want 500", rec.Code)
	}
	out := buf.String()
	for _, want := range []string{"Panic recovered", "boom", "path=/explode", "method=GET", "stack"} {
		if !strings.Contains(out, want) {
			t.Fatalf("log missing %q: %q", want, out)
		}
	}
}

func TestNewRecover_includesRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	// NewRequestID sets the context value that NewRecover logs.
	h := NewRequestID(NewRecover(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("kaboom")
	})))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "trace-9")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(buf.String(), "request_id=trace-9") {
		t.Fatalf("expected request_id in panic log: %q", buf.String())
	}
}

func TestNewRecover_reraisesAbortHandler(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	h := NewRecover(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	defer func() {
		rec := recover()
		if rec != http.ErrAbortHandler {
			t.Fatalf("expected ErrAbortHandler to propagate, got %v", rec)
		}
		if buf.Len() != 0 {
			t.Fatalf("ErrAbortHandler should not be logged, got: %q", buf.String())
		}
	}()

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	t.Fatal("expected ErrAbortHandler to be re-panicked")
}

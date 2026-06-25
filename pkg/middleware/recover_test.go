package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRecover_noPanic(t *testing.T) {
	log, buf := testLogger(t)

	called := false
	h := NewRecover(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !called {
		t.Fatal("next handler was not called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want 200", rec.Code)
	}
	if strings.Contains(buf.String(), "Panic recovered") {
		t.Fatalf("unexpected panic log: %q", buf.String())
	}
}

func TestNewRecover_panic(t *testing.T) {
	log, buf := testLogger(t)

	h := NewRecover(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/panic", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want 500", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"message":"Internal error"}` {
		t.Fatalf("body: got %q want internal error json", got)
	}
	out := buf.String()
	if !strings.Contains(out, "Panic recovered") {
		t.Fatalf("expected panic log, got %q", out)
	}
	if !strings.Contains(out, `"method":"POST"`) || !strings.Contains(out, `"path":"/panic"`) {
		t.Fatalf("expected request attrs in panic log, got %q", out)
	}
}

func TestNewRecover_errAbortHandler(t *testing.T) {
	log, _ := testLogger(t)

	defer func() {
		if rec := recover(); rec != http.ErrAbortHandler {
			t.Fatalf("expected http.ErrAbortHandler re-panic, got %v", rec)
		}
	}()

	h := NewRecover(log, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

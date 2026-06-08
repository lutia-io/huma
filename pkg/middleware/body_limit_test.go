package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewBodySizeLimit_disabled(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	// maxBytes <= 0 should return the same handler, unwrapped.
	if got := NewBodySizeLimit(0, inner); got == nil {
		t.Fatal("expected handler, got nil")
	}

	var readErr error
	var n int
	h := NewBodySizeLimit(0, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		n, readErr = len(b), err
	}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789"))
	h.ServeHTTP(httptest.NewRecorder(), req)

	if readErr != nil {
		t.Fatalf("unexpected read error: %v", readErr)
	}
	if n != 10 {
		t.Fatalf("read %d bytes, want 10", n)
	}
}

func TestNewBodySizeLimit_underLimit(t *testing.T) {
	var readErr error
	h := NewBodySizeLimit(16, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789"))
	h.ServeHTTP(httptest.NewRecorder(), req)

	if readErr != nil {
		t.Fatalf("unexpected read error under limit: %v", readErr)
	}
}

func TestNewBodySizeLimit_overLimit(t *testing.T) {
	var readErr error
	h := NewBodySizeLimit(4, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("0123456789"))
	h.ServeHTTP(httptest.NewRecorder(), req)

	if readErr == nil {
		t.Fatal("expected read error when body exceeds limit")
	}
	var maxErr *http.MaxBytesError
	if !errors.As(readErr, &maxErr) {
		t.Fatalf("expected *http.MaxBytesError, got %T: %v", readErr, readErr)
	}
}

func TestNewBodySizeLimit_nilBody(t *testing.T) {
	called := false
	h := NewBodySizeLimit(8, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Body = nil // exercise the nil-body guard
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !called {
		t.Fatal("next handler was not called for nil body")
	}
}

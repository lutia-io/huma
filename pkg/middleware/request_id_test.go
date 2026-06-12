package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lutia-io/huma/pkg/requestid"
)

func TestNewRequestID_generates(t *testing.T) {
	var ctxID string
	h := NewRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID, _ = RequestIDFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if len(ctxID) != 32 {
		t.Fatalf("generated id length: got %d (%q) want 32", len(ctxID), ctxID)
	}
	if got := rec.Header().Get(RequestIDHeader); got != ctxID {
		t.Fatalf("response header %q != context id %q", got, ctxID)
	}
}

func TestNewRequestID_ignoresClientHeader(t *testing.T) {
	var ctxID string
	h := NewRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID, _ = RequestIDFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "client-supplied-id")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if ctxID == "client-supplied-id" {
		t.Fatalf("expected server-generated id, got client value %q", ctxID)
	}
	if len(ctxID) != 32 {
		t.Fatalf("generated id length: got %d (%q) want 32", len(ctxID), ctxID)
	}
	if got := rec.Header().Get(RequestIDHeader); got != ctxID {
		t.Fatalf("response header %q != context id %q", got, ctxID)
	}
}

func TestRequestIDFromContext_absent(t *testing.T) {
	if id, ok := RequestIDFromContext(context.Background()); ok || id != "" {
		t.Fatalf("expected empty/false, got %q/%v", id, ok)
	}
}

func TestRequestIDFromContext_presentButEmpty(t *testing.T) {
	ctx := requestid.WithContext(context.Background(), "")
	if id, ok := RequestIDFromContext(ctx); ok || id != "" {
		t.Fatalf("expected empty/false for empty value, got %q/%v", id, ok)
	}
}

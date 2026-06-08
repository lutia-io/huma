package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRequestID_reusesValidClientID(t *testing.T) {
	var ctxID string
	var ctxOK bool
	h := NewRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID, ctxOK = RequestIDFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "valid_ID-1.2")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if !ctxOK || ctxID != "valid_ID-1.2" {
		t.Fatalf("context id: got %q (ok=%v) want valid_ID-1.2", ctxID, ctxOK)
	}
	if got := rec.Header().Get(RequestIDHeader); got != "valid_ID-1.2" {
		t.Fatalf("response header: got %q want valid_ID-1.2", got)
	}
}

func TestNewRequestID_generatesWhenInvalid(t *testing.T) {
	tests := []struct {
		name   string
		header string
		set    bool
	}{
		{"missing header", "", false},
		{"empty header", "", true},
		{"too long", strings.Repeat("a", maxRequestIDLen+1), true},
		{"illegal char", "bad id with spaces", true},
		{"illegal slash", "a/b", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctxID string
			h := NewRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctxID, _ = RequestIDFromContext(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.set {
				req.Header.Set(RequestIDHeader, tt.header)
			}
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if ctxID == tt.header && tt.header != "" {
				t.Fatalf("expected generated id, but got client value %q", ctxID)
			}
			if len(ctxID) != 32 { // 16 random bytes hex-encoded
				t.Fatalf("generated id length: got %d (%q) want 32", len(ctxID), ctxID)
			}
			if rec.Header().Get(RequestIDHeader) != ctxID {
				t.Fatalf("response header %q != context id %q", rec.Header().Get(RequestIDHeader), ctxID)
			}
		})
	}
}

func TestRequestIDFromContext_absent(t *testing.T) {
	if id, ok := RequestIDFromContext(context.Background()); ok || id != "" {
		t.Fatalf("expected empty/false, got %q/%v", id, ok)
	}
}

func TestRequestIDFromContext_presentButEmpty(t *testing.T) {
	// A stored empty string must still report ok=false.
	ctx := context.WithValue(context.Background(), requestIDKey{}, "")
	if id, ok := RequestIDFromContext(ctx); ok || id != "" {
		t.Fatalf("expected empty/false for empty value, got %q/%v", id, ok)
	}
}

func TestIsValidRequestID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"abcXYZ", true},
		{"0123456789", true},
		{"a-b_c.d", true},
		{"", false},
		{strings.Repeat("x", maxRequestIDLen), true},
		{strings.Repeat("x", maxRequestIDLen+1), false},
		{"has space", false},
		{"new\nline", false},
		{"semi;colon", false},
	}
	for _, tt := range tests {
		if got := isValidRequestID(tt.id); got != tt.want {
			t.Errorf("isValidRequestID(%q) = %v, want %v", tt.id, got, tt.want)
		}
	}
}

func TestNewRequestID_randFallback(t *testing.T) {
	orig := randRead
	randRead = func(b []byte) (int, error) { return 0, errors.New("no entropy") }
	defer func() { randRead = orig }()

	id := newRequestID()
	if len(id) != 16 { // 8-byte fallback hex-encoded
		t.Fatalf("fallback id length: got %d (%q) want 16", len(id), id)
	}
}

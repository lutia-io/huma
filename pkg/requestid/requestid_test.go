package requestid

import (
	"context"
	"errors"
	"testing"
)

func TestFromContext(t *testing.T) {
	if id, ok := FromContext(context.Background()); ok || id != "" {
		t.Fatalf("absent: got %q/%v want empty/false", id, ok)
	}

	ctx := WithContext(context.Background(), "abc123")
	if id, ok := FromContext(ctx); !ok || id != "abc123" {
		t.Fatalf("present: got %q/%v want abc123/true", id, ok)
	}

	ctx = WithContext(context.Background(), "")
	if id, ok := FromContext(ctx); ok || id != "" {
		t.Fatalf("empty: got %q/%v want empty/false", id, ok)
	}
}

func TestNew(t *testing.T) {
	id := New()
	if len(id) != 32 { // 16-byte ID hex-encoded
		t.Fatalf("id length: got %d (%q) want 32", len(id), id)
	}
}

func TestNew_randFallback(t *testing.T) {
	orig := randRead
	randRead = func(b []byte) (int, error) { return 0, errors.New("no entropy") }
	defer func() { randRead = orig }()

	id := New()
	if len(id) != 16 { // 8-byte fallback hex-encoded
		t.Fatalf("fallback id length: got %d (%q) want 16", len(id), id)
	}
}

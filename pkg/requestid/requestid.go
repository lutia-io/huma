package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type key struct{}

// randRead is indirected through a variable so tests can simulate a failing
// entropy source.
var randRead = rand.Read

// New returns a random hex-encoded ID. crypto/rand essentially never fails, but
// if it does we fall back to a shorter ID rather than panicking.
func New() string {
	var b [16]byte
	_, err := randRead(b[:])
	if err != nil {
		var b2 [8]byte
		_, _ = randRead(b2[:])
		return hex.EncodeToString(b2[:])
	}
	return hex.EncodeToString(b[:])
}

// WithContext returns a copy of ctx that carries id.
func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, key{}, id)
}

// FromContext returns the request ID stored in ctx, if any. The boolean is
// false when no ID is present.
func FromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(key{}).(string)
	if !ok || id == "" {
		return "", false
	}
	return id, true
}

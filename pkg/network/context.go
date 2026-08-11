package network

import (
	"context"
	"net/http"
	"strings"

	"github.com/lutia-io/huma/pkg/uuid"
)

const HeaderNetworkID = "X-Network-Id"

type contextKey struct{}

// Context carries the network for the current request.
type Context struct {
	NetworkID string
}

// WithContext returns a copy of ctx that carries NetworkContext.
func WithContext(ctx context.Context, nc Context) context.Context {
	return context.WithValue(ctx, contextKey{}, nc)
}

// FromContext returns the NetworkContext stored in ctx, if any.
func FromContext(ctx context.Context) (Context, bool) {
	nc, ok := ctx.Value(contextKey{}).(Context)
	if !ok || nc.NetworkID == "" {
		return Context{}, false
	}
	return nc, true
}

// Resolve builds a NetworkContext from an explicit body value, else the
// X-Network-Id header. bodyNetworkID should be the networkId from a decoded
// JSON body when present; pass "" to use the header only.
func Resolve(r *http.Request, bodyNetworkID string) (Context, bool) {
	id := strings.TrimSpace(bodyNetworkID)
	if id == "" {
		id = strings.TrimSpace(r.Header.Get(HeaderNetworkID))
	}
	if id == "" || !uuid.Valid(id) {
		return Context{}, false
	}
	return Context{NetworkID: id}, true
}

// Middleware attaches NetworkContext from X-Network-Id when present.
// Body fields are resolved in handlers via Resolve.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if nc, ok := Resolve(r, ""); ok {
			r = r.WithContext(WithContext(r.Context(), nc))
		}
		next.ServeHTTP(w, r)
	})
}

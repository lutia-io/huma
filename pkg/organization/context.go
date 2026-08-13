package organization

import (
	"context"
	"net/http"
	"strings"

	"github.com/lutia-io/huma/pkg/principal"
	"github.com/lutia-io/huma/pkg/uuid"
)

const HeaderOrganizationID = "X-Organization-Id"

type contextKey struct{}

// Context carries the organization for the current request.
type Context struct {
	OrganizationID string
}

// WithContext returns a copy of ctx that carries OrganizationContext.
func WithContext(ctx context.Context, oc Context) context.Context {
	return context.WithValue(ctx, contextKey{}, oc)
}

// FromContext returns the OrganizationContext stored in ctx, if any.
func FromContext(ctx context.Context) (Context, bool) {
	oc, ok := ctx.Value(contextKey{}).(Context)
	if !ok || oc.OrganizationID == "" {
		return Context{}, false
	}
	return oc, true
}

// Resolve builds an OrganizationContext from an explicit body value, else the
// X-Organization-Id header. bodyOrganizationID should be the organizationId
// from a decoded JSON body when present; pass "" to use the header only.
func Resolve(r *http.Request, bodyOrganizationID string) (Context, bool) {
	id := strings.TrimSpace(bodyOrganizationID)
	if id == "" {
		id = strings.TrimSpace(r.Header.Get(HeaderOrganizationID))
	}
	if id == "" || !uuid.Valid(id) {
		return Context{}, false
	}
	return Context{OrganizationID: id}, true
}

// ResolveID prefers principal.OrganizationID, then request OrganizationContext,
// then body/header via Resolve.
func ResolveID(r *http.Request, p principal.Principal, bodyOrganizationID string) (string, bool) {
	if p.OrganizationID != "" {
		return p.OrganizationID, true
	}
	if oc, ok := FromContext(r.Context()); ok {
		return oc.OrganizationID, true
	}
	oc, ok := Resolve(r, bodyOrganizationID)
	if !ok {
		return "", false
	}
	return oc.OrganizationID, true
}

// Middleware attaches OrganizationContext from X-Organization-Id when present.
// Body fields are resolved in handlers via Resolve.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if oc, ok := Resolve(r, ""); ok {
			r = r.WithContext(WithContext(r.Context(), oc))
		}
		next.ServeHTTP(w, r)
	})
}

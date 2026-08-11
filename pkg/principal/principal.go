package principal

import (
	"context"

	"github.com/lutia-io/huma/pkg/apperror"
)

type Type string

const (
	TypeUser             Type = "user"
	TypeOrganizationUser Type = "organization_user"
)

type contextKey struct{}

// Principal is the authenticated caller.
type Principal struct {
	Type           Type
	ID             string
	NetworkID      string
	OrganizationID string // empty for platform users
}

// WithContext returns a copy of ctx that carries p.
func WithContext(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

// FromContext returns the Principal stored in ctx, if any.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(contextKey{}).(Principal)
	if !ok || p.ID == "" {
		return Principal{}, false
	}
	return p, true
}

// RequireUser ensures the caller is a platform user. When the access token is
// network-scoped (NetworkID set) and networkID is provided, they must match.
// Unscoped user tokens (no nid) may act with a request NetworkContext; ownership
// of that network is enforced separately where needed.
func RequireUser(p Principal, networkID string) error {
	if p.Type != TypeUser {
		return apperror.NewUnauthorizedError("Platform user authentication required", nil)
	}
	if p.NetworkID != "" && networkID != "" && p.NetworkID != networkID {
		return apperror.NewUnauthorizedError("Network mismatch", nil)
	}
	return nil
}

// RequireOrganizationUser ensures the caller is an organization user whose
// token matches the given network and organization.
func RequireOrganizationUser(p Principal, networkID, organizationID string) error {
	if p.Type != TypeOrganizationUser {
		return apperror.NewUnauthorizedError("Organization user authentication required", nil)
	}
	if networkID != "" && p.NetworkID != networkID {
		return apperror.NewUnauthorizedError("Network mismatch", nil)
	}
	if organizationID != "" && p.OrganizationID != organizationID {
		return apperror.NewUnauthorizedError("Organization mismatch", nil)
	}
	return nil
}

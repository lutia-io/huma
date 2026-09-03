package organizationuser

import (
	"context"
	"testing"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/principal"
)

func TestResolveCreateActor(t *testing.T) {
	ctx := context.Background()
	owned := &Scope{
		ID:             "ou-1",
		OrganizationID: "org-1",
		NetworkID:      "net-1",
		UserID:         "user-1",
	}
	lookup := func(_ context.Context, id string) (*Scope, error) {
		if id == owned.ID {
			return owned, nil
		}
		return nil, apperror.NewNotFoundError("Organization user not found", nil)
	}

	t.Run("organization user creates as self", func(t *testing.T) {
		scope, err := ResolveCreateActor(ctx, principal.Principal{
			Type:           principal.TypeOrganizationUser,
			ID:             "ou-1",
			NetworkID:      "net-1",
			OrganizationID: "org-1",
		}, "", lookup)
		if err != nil {
			t.Fatal(err)
		}
		if scope.ID != "ou-1" || scope.OrganizationID != "org-1" || scope.NetworkID != "net-1" {
			t.Fatalf("got %+v", scope)
		}
	})

	t.Run("organization user matching id", func(t *testing.T) {
		_, err := ResolveCreateActor(ctx, principal.Principal{
			Type:           principal.TypeOrganizationUser,
			ID:             "ou-1",
			NetworkID:      "net-1",
			OrganizationID: "org-1",
		}, "ou-1", lookup)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("organization user cannot impersonate", func(t *testing.T) {
		_, err := ResolveCreateActor(ctx, principal.Principal{
			Type:           principal.TypeOrganizationUser,
			ID:             "ou-1",
			NetworkID:      "net-1",
			OrganizationID: "org-1",
		}, "ou-2", lookup)
		if !apperror.IsForbidden(err) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("organization user missing scope", func(t *testing.T) {
		_, err := ResolveCreateActor(ctx, principal.Principal{
			Type: principal.TypeOrganizationUser,
			ID:   "ou-1",
		}, "", lookup)
		if !apperror.IsForbidden(err) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("platform user requires organization user id", func(t *testing.T) {
		_, err := ResolveCreateActor(ctx, principal.Principal{
			Type: principal.TypeUser,
			ID:   "user-1",
		}, "", lookup)
		if !apperror.IsBadRequest(err) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("platform user unknown organization user", func(t *testing.T) {
		_, err := ResolveCreateActor(ctx, principal.Principal{
			Type: principal.TypeUser,
			ID:   "user-1",
		}, "missing", lookup)
		if !apperror.IsNotFound(err) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("platform user other owner", func(t *testing.T) {
		_, err := ResolveCreateActor(ctx, principal.Principal{
			Type: principal.TypeUser,
			ID:   "user-2",
		}, "ou-1", lookup)
		if !apperror.IsNotFound(err) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("platform user network mismatch", func(t *testing.T) {
		_, err := ResolveCreateActor(ctx, principal.Principal{
			Type:      principal.TypeUser,
			ID:        "user-1",
			NetworkID: "net-other",
		}, "ou-1", lookup)
		if !apperror.IsForbidden(err) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("platform user creates as administered user", func(t *testing.T) {
		scope, err := ResolveCreateActor(ctx, principal.Principal{
			Type: principal.TypeUser,
			ID:   "user-1",
		}, "ou-1", lookup)
		if err != nil {
			t.Fatal(err)
		}
		if scope != owned {
			t.Fatalf("got %+v", scope)
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		_, err := ResolveCreateActor(ctx, principal.Principal{}, "ou-1", lookup)
		if !apperror.IsUnauthorized(err) {
			t.Fatalf("got %v", err)
		}
	})
}

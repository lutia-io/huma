package organizationuser

import (
	"context"
	"io"
	"testing"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
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

type stubHasher struct{}

func (stubHasher) Hash(text string) (string, error) { return "hashed:" + text, nil }
func (stubHasher) Compare(text, encoded string) (bool, error) {
	return encoded == "hashed:"+text, nil
}

type memoryStore struct {
	systemID string
	inserted *organizationUser
}

func (m *memoryStore) Insert(context.Context, *organizationUser) (string, error) { return "", nil }
func (m *memoryStore) Update(context.Context, *organizationUser, *string) error  { return nil }
func (m *memoryStore) GetByID(context.Context, string) (*organizationUser, error) {
	return nil, apperror.NewNotFoundError("Organization user not found", nil)
}
func (m *memoryStore) GetByEmail(context.Context, string, string, string) (*organizationUser, error) {
	return nil, apperror.NewNotFoundError("Organization user not found", nil)
}
func (m *memoryStore) GetPasswordByID(context.Context, string) (string, error) { return "", nil }
func (m *memoryStore) UpdatePassword(context.Context, string, string) error    { return nil }
func (m *memoryStore) GetSystemUserID(_ context.Context, _, _ string) (string, error) {
	if m.systemID == "" {
		return "", apperror.NewNotFoundError("Organization user not found", nil)
	}
	return m.systemID, nil
}
func (m *memoryStore) InsertSystemUser(_ context.Context, u *organizationUser) (string, error) {
	m.inserted = u
	u.ID = "sys-1"
	m.systemID = u.ID
	return u.ID, nil
}
func (m *memoryStore) List(context.Context, listParams) (*listResult, error) { return nil, nil }

func TestEnsureSystemUser(t *testing.T) {
	orgID := "00000000-0000-4000-8000-000000000001"
	netID := "00000000-0000-4000-8000-000000000002"

	t.Run("returns existing", func(t *testing.T) {
		store := &memoryStore{systemID: "sys-existing"}
		s := NewService(logger.NewWithWriter(io.Discard), store, stubHasher{})
		id, err := s.EnsureSystemUser(context.Background(), orgID, netID)
		if err != nil {
			t.Fatal(err)
		}
		if id != "sys-existing" {
			t.Fatalf("got %s", id)
		}
		if store.inserted != nil {
			t.Fatal("should not insert when system user exists")
		}
	})

	t.Run("inserts when missing", func(t *testing.T) {
		store := &memoryStore{}
		s := NewService(logger.NewWithWriter(io.Discard), store, stubHasher{})
		id, err := s.EnsureSystemUser(context.Background(), orgID, netID)
		if err != nil {
			t.Fatal(err)
		}
		if id != "sys-1" {
			t.Fatalf("got %s", id)
		}
		if store.inserted == nil || !store.inserted.Internal {
			t.Fatalf("inserted %#v", store.inserted)
		}
		if store.inserted.Email != systemUserEmail(orgID) {
			t.Fatalf("email %s", store.inserted.Email)
		}
		if store.inserted.FirstName != systemUserFirstName || store.inserted.LastName != systemUserLastName {
			t.Fatalf("name %s %s", store.inserted.FirstName, store.inserted.LastName)
		}
	})

	t.Run("rejects invalid ids", func(t *testing.T) {
		s := NewService(logger.NewWithWriter(io.Discard), &memoryStore{}, stubHasher{})
		if _, err := s.EnsureSystemUser(context.Background(), "bad", netID); !apperror.IsBadRequest(err) {
			t.Fatalf("got %v", err)
		}
	})
}

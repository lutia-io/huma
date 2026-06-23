package user

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/lutia-io/huma/pkg/logger"
)

type fakeStore struct {
	findUsers      []user
	findErr        error
	findByIDUser   *user
	findByIDErr    error
	insertID       string
	insertErr      error
	updateErr      error
	softDeleteErr  error
	insertedUser   *user
	updatedID      string
	updatedUser    *user
	softDeletedID  string
	findByIDCalled bool
}

func (f *fakeStore) Find(ctx context.Context) ([]user, error) {
	return f.findUsers, f.findErr
}

func (f *fakeStore) FindByID(ctx context.Context, id string) (*user, error) {
	f.findByIDCalled = true
	return f.findByIDUser, f.findByIDErr
}

func (f *fakeStore) Insert(ctx context.Context, user *user) (string, error) {
	f.insertedUser = user
	return f.insertID, f.insertErr
}

func (f *fakeStore) UpdateByID(ctx context.Context, id string, user *user) error {
	f.updatedID = id
	f.updatedUser = user
	return f.updateErr
}

func (f *fakeStore) SoftDeleteByID(ctx context.Context, id string) error {
	f.softDeletedID = id
	return f.softDeleteErr
}

type fakeHasher struct {
	hash   string
	err    error
	input  string
	called bool
}

func (f *fakeHasher) Hash(text string) (string, error) {
	f.called = true
	f.input = text
	return f.hash, f.err
}

func newTestService(store store, hasher *fakeHasher) *service {
	return newService(logger.NewWithWriter(io.Discard), store, hasher)
}

func TestNewService(t *testing.T) {
	store := &fakeStore{}
	hasher := &fakeHasher{}
	log := logger.NewWithWriter(io.Discard)

	got := newService(log, store, hasher)
	if got.logger != log {
		t.Fatal("logger was not assigned")
	}
	if got.store != store {
		t.Fatal("store was not assigned")
	}
	if got.hasher != hasher {
		t.Fatal("hasher was not assigned")
	}
}

func TestService_Find(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		want := []user{{ID: "user-1", Email: "ada@example.com"}}
		store := &fakeStore{findUsers: want}
		service := newTestService(store, &fakeHasher{})

		got, err := service.Find(context.Background())
		if err != nil {
			t.Fatalf("Find: %v", err)
		}
		if len(got) != 1 || got[0] != want[0] {
			t.Fatalf("users: got %#v, want %#v", got, want)
		}
	})

	t.Run("store error", func(t *testing.T) {
		wantErr := errors.New("find failed")
		service := newTestService(&fakeStore{findErr: wantErr}, &fakeHasher{})

		got, err := service.Find(context.Background())
		if !errors.Is(err, wantErr) {
			t.Fatalf("err: got %v, want %v", err, wantErr)
		}
		if got != nil {
			t.Fatalf("users: got %#v, want nil", got)
		}
	})
}

func TestService_FindByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		want := &user{ID: "user-1", Email: "ada@example.com"}
		service := newTestService(&fakeStore{findByIDUser: want}, &fakeHasher{})

		got, err := service.FindByID(context.Background(), "user-1")
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if got != want {
			t.Fatalf("user: got %#v, want %#v", got, want)
		}
	})

	t.Run("store error", func(t *testing.T) {
		wantErr := errors.New("find by id failed")
		service := newTestService(&fakeStore{findByIDErr: wantErr}, &fakeHasher{})

		got, err := service.FindByID(context.Background(), "user-1")
		if !errors.Is(err, wantErr) {
			t.Fatalf("err: got %v, want %v", err, wantErr)
		}
		if got != nil {
			t.Fatalf("user: got %#v, want nil", got)
		}
	})
}

func TestService_InsertValidation(t *testing.T) {
	tests := []struct {
		name string
		req  insertUserRequest
		msg  string
	}{
		{
			name: "empty first name",
			req:  insertUserRequest{LastName: "Lovelace", Email: "ada@example.com", Password: "pw"},
			msg:  "First name is required",
		},
		{
			name: "empty last name",
			req:  insertUserRequest{FirstName: "Ada", Email: "ada@example.com", Password: "pw"},
			msg:  "Last name is required",
		},
		{
			name: "empty email",
			req:  insertUserRequest{FirstName: "Ada", LastName: "Lovelace", Password: "pw"},
			msg:  "Email is required",
		},
		{
			name: "invalid email",
			req:  insertUserRequest{FirstName: "Ada", LastName: "Lovelace", Email: "not-an-email", Password: "pw"},
			msg:  "Email is invalid",
		},
		{
			name: "empty password",
			req:  insertUserRequest{FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.com"},
			msg:  "Password is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasher := &fakeHasher{hash: "hashed-password"}
			got, err := newTestService(&fakeStore{}, hasher).Insert(context.Background(), tt.req)

			if got != "" {
				t.Fatalf("id: got %q, want empty", got)
			}
			requireAppError(t, err, apperror.ErrorVariantBadRequest, "user.service.Insert")
			var appErr *apperror.Error
			if !errors.As(err, &appErr) || appErr.Msg != tt.msg {
				t.Fatalf("msg: got %q, want %q", appErr.Msg, tt.msg)
			}
			if hasher.called {
				t.Fatal("hasher should not be called for invalid requests")
			}
		})
	}
}

func TestService_Insert(t *testing.T) {
	t.Run("success trims input, hashes password, and stores user", func(t *testing.T) {
		store := &fakeStore{insertID: "user-1"}
		hasher := &fakeHasher{hash: "hashed-password"}
		service := newTestService(store, hasher)

		before := time.Now().UTC()
		got, err := service.Insert(context.Background(), insertUserRequest{
			FirstName: "  Ada  ",
			LastName:  "  Lovelace  ",
			Email:     "  ada@example.com  ",
			Password:  "plain-password",
		})
		after := time.Now().UTC()

		if err != nil {
			t.Fatalf("Insert: %v", err)
		}
		if got != "user-1" {
			t.Fatalf("id: got %q, want user-1", got)
		}
		if !hasher.called || hasher.input != "plain-password" {
			t.Fatalf("hasher input: called=%v input=%q", hasher.called, hasher.input)
		}
		if store.insertedUser == nil {
			t.Fatal("store did not receive a user")
		}
		if store.insertedUser.FirstName != "Ada" || store.insertedUser.LastName != "Lovelace" || store.insertedUser.Email != "ada@example.com" {
			t.Fatalf("stored user was not trimmed: %#v", store.insertedUser)
		}
		if store.insertedUser.Password != "hashed-password" {
			t.Fatalf("password: got %q, want hashed-password", store.insertedUser.Password)
		}
		if store.insertedUser.CreatedAt.Before(before) || store.insertedUser.CreatedAt.After(after) {
			t.Fatalf("created_at %v outside [%v, %v]", store.insertedUser.CreatedAt, before, after)
		}
		if !store.insertedUser.UpdatedAt.Equal(store.insertedUser.CreatedAt) {
			t.Fatalf("updated_at: got %v, want %v", store.insertedUser.UpdatedAt, store.insertedUser.CreatedAt)
		}
	})

	t.Run("hash error", func(t *testing.T) {
		hasher := &fakeHasher{err: errors.New("hash failed")}
		got, err := newTestService(&fakeStore{}, hasher).Insert(context.Background(), validInsertRequest())

		if got != "" {
			t.Fatalf("id: got %q, want empty", got)
		}
		requireAppError(t, err, apperror.ErrorVariantInternal, "user.service.Insert")
	})

	t.Run("store conflict error", func(t *testing.T) {
		store := &fakeStore{insertErr: apperror.NewConflictError("user.store.Insert", "User already exists", nil)}
		got, err := newTestService(store, &fakeHasher{hash: "hashed-password"}).Insert(context.Background(), validInsertRequest())

		if got != "" {
			t.Fatalf("id: got %q, want empty", got)
		}
		requireAppError(t, err, apperror.ErrorVariantConflict, "user.store.Insert")
	})

	t.Run("store internal error", func(t *testing.T) {
		wantErr := errors.New("insert failed")
		store := &fakeStore{insertErr: wantErr}
		got, err := newTestService(store, &fakeHasher{hash: "hashed-password"}).Insert(context.Background(), validInsertRequest())

		if got != "" {
			t.Fatalf("id: got %q, want empty", got)
		}
		if !errors.Is(err, wantErr) {
			t.Fatalf("err: got %v, want %v", err, wantErr)
		}
	})
}

func TestService_UpdateByID(t *testing.T) {
	t.Run("find error", func(t *testing.T) {
		wantErr := errors.New("find failed")
		store := &fakeStore{findByIDErr: wantErr}
		err := newTestService(store, &fakeHasher{}).UpdateByID(context.Background(), "user-1", validUpdateRequest())

		if !errors.Is(err, wantErr) {
			t.Fatalf("err: got %v, want %v", err, wantErr)
		}
		if store.updatedUser != nil {
			t.Fatal("store update should not be called")
		}
	})

	t.Run("empty first name", func(t *testing.T) {
		store := &fakeStore{findByIDUser: &user{ID: "user-1"}}
		err := newTestService(store, &fakeHasher{}).UpdateByID(context.Background(), "user-1", updateUserRequest{LastName: "Lovelace"})

		requireAppError(t, err, apperror.ErrorVariantBadRequest, "user.service.Update")
		if store.updatedUser != nil {
			t.Fatal("store update should not be called")
		}
	})

	t.Run("empty last name", func(t *testing.T) {
		store := &fakeStore{findByIDUser: &user{ID: "user-1"}}
		err := newTestService(store, &fakeHasher{}).UpdateByID(context.Background(), "user-1", updateUserRequest{FirstName: "Ada"})

		requireAppError(t, err, apperror.ErrorVariantBadRequest, "user.service.Update")
		if store.updatedUser != nil {
			t.Fatal("store update should not be called")
		}
	})

	t.Run("update error", func(t *testing.T) {
		wantErr := errors.New("update failed")
		store := &fakeStore{findByIDUser: &user{ID: "user-1"}, updateErr: wantErr}
		err := newTestService(store, &fakeHasher{}).UpdateByID(context.Background(), "user-1", validUpdateRequest())

		if !errors.Is(err, wantErr) {
			t.Fatalf("err: got %v, want %v", err, wantErr)
		}
	})

	t.Run("success trims names and updates timestamp", func(t *testing.T) {
		existing := &user{
			ID:        "user-1",
			FirstName: "Ada",
			LastName:  "Lovelace",
			Email:     "ada@example.com",
			Password:  "hashed-password",
			UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		}
		store := &fakeStore{findByIDUser: existing}
		service := newTestService(store, &fakeHasher{})

		before := time.Now().UTC()
		err := service.UpdateByID(context.Background(), "user-1", updateUserRequest{
			FirstName: "  Augusta  ",
			LastName:  "  Byron  ",
		})
		after := time.Now().UTC()

		if err != nil {
			t.Fatalf("UpdateByID: %v", err)
		}
		if store.updatedID != "user-1" {
			t.Fatalf("updated id: got %q, want user-1", store.updatedID)
		}
		if store.updatedUser != existing {
			t.Fatal("service should update the fetched user")
		}
		if existing.FirstName != "Augusta" || existing.LastName != "Byron" {
			t.Fatalf("name: got %q %q", existing.FirstName, existing.LastName)
		}
		if existing.UpdatedAt.Before(before) || existing.UpdatedAt.After(after) {
			t.Fatalf("updated_at %v outside [%v, %v]", existing.UpdatedAt, before, after)
		}
	})
}

func TestService_DeleteByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := &fakeStore{}
		err := newTestService(store, &fakeHasher{}).DeleteByID(context.Background(), "user-1")

		if err != nil {
			t.Fatalf("DeleteByID: %v", err)
		}
		if store.softDeletedID != "user-1" {
			t.Fatalf("soft deleted id: got %q, want user-1", store.softDeletedID)
		}
	})

	t.Run("store error", func(t *testing.T) {
		wantErr := errors.New("delete failed")
		store := &fakeStore{softDeleteErr: wantErr}
		err := newTestService(store, &fakeHasher{}).DeleteByID(context.Background(), "user-1")

		if !errors.Is(err, wantErr) {
			t.Fatalf("err: got %v, want %v", err, wantErr)
		}
	})
}

func validInsertRequest() insertUserRequest {
	return insertUserRequest{
		FirstName: "Ada",
		LastName:  "Lovelace",
		Email:     "ada@example.com",
		Password:  "plain-password",
	}
}

func validUpdateRequest() updateUserRequest {
	return updateUserRequest{
		FirstName: "Ada",
		LastName:  "Lovelace",
	}
}

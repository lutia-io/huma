package user

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lutia-io/huma/pkg/apperror"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestMongoStore(t *testing.T) {
	ctx := context.Background()
	container, err := mongodb.Run(ctx, "mongo:6")
	if err != nil {
		t.Fatalf("start mongo container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Fatalf("terminate mongo container: %v", err)
		}
	})

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("mongo connection string: %v", err)
	}

	client := newMongoClient(t, ctx, uri)
	t.Cleanup(func() {
		if err := client.Disconnect(ctx); err != nil {
			t.Fatalf("disconnect mongo client: %v", err)
		}
	})

	t.Run("newMongoStore registers unique email index", func(t *testing.T) {
		store := resetStore(t, ctx, client)
		if store.client != client {
			t.Fatal("newMongoStore did not retain the mongo client")
		}

		first := validUser("Ada", "Lovelace", "ada@example.com")
		if _, err := store.Insert(ctx, first); err != nil {
			t.Fatalf("insert first user: %v", err)
		}
		_, err := store.Insert(ctx, validUser("Ada", "Byron", "ada@example.com"))
		requireAppError(t, err, apperror.ErrorVariantConflict, "user.store.Insert")
	})

	t.Run("newMongoStore returns index registration error", func(t *testing.T) {
		badClient := mustConnectClient(t, options.Client().
			ApplyURI("mongodb://127.0.0.1:1").
			SetServerSelectionTimeout(10*time.Millisecond).
			SetBSONOptions(&options.BSONOptions{ObjectIDAsHexString: true}))
		t.Cleanup(func() { _ = badClient.Disconnect(ctx) })

		got, err := newMongoStore(badClient)
		if err == nil {
			t.Fatal("expected index registration error")
		}
		if got != nil {
			t.Fatalf("store: got %#v, want nil", got)
		}
	})

	t.Run("Find returns active users only", func(t *testing.T) {
		store := resetStore(t, ctx, client)
		activeID, err := store.Insert(ctx, validUser("Ada", "Lovelace", "ada@example.com"))
		if err != nil {
			t.Fatalf("insert active user: %v", err)
		}
		deletedID, err := store.Insert(ctx, validUser("Grace", "Hopper", "grace@example.com"))
		if err != nil {
			t.Fatalf("insert deleted user: %v", err)
		}
		if err := store.SoftDeleteByID(ctx, deletedID); err != nil {
			t.Fatalf("soft delete user: %v", err)
		}

		got, err := store.Find(ctx)
		if err != nil {
			t.Fatalf("Find: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("users: got %d, want 1 (%#v)", len(got), got)
		}
		if got[0].ID != activeID || got[0].Email != "ada@example.com" {
			t.Fatalf("user: got %#v, want active user %q", got[0], activeID)
		}
	})

	t.Run("Find returns decode errors", func(t *testing.T) {
		store := resetStore(t, ctx, client)
		collection(t, client).InsertOne(ctx, bson.M{
			"first_name": "Bad",
			"last_name":  "Data",
			"email":      "bad@example.com",
			"password":   "hashed",
			"created_at": "not-a-time",
			"updated_at": time.Now().UTC(),
		})

		got, err := store.Find(ctx)
		if got != nil {
			t.Fatalf("users: got %#v, want nil", got)
		}
		requireAppError(t, err, apperror.ErrorVariantInternal, "user.store.Find")
	})

	t.Run("Find returns query errors", func(t *testing.T) {
		store := disconnectedStore(t, ctx, uri)

		got, err := store.Find(ctx)
		if got != nil {
			t.Fatalf("users: got %#v, want nil", got)
		}
		requireAppError(t, err, apperror.ErrorVariantInternal, "user.store.Find")
	})

	t.Run("FindByID returns matching active user", func(t *testing.T) {
		store := resetStore(t, ctx, client)
		id, err := store.Insert(ctx, validUser("Ada", "Lovelace", "ada@example.com"))
		if err != nil {
			t.Fatalf("insert user: %v", err)
		}

		got, err := store.FindByID(ctx, id)
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if got.ID != id || got.Email != "ada@example.com" {
			t.Fatalf("user: got %#v, want id %q", got, id)
		}
	})

	t.Run("FindByID returns not found for invalid id", func(t *testing.T) {
		store := resetStore(t, ctx, client)

		got, err := store.FindByID(ctx, "not-an-object-id")
		if got != nil {
			t.Fatalf("user: got %#v, want nil", got)
		}
		requireAppError(t, err, apperror.ErrorVariantNotFound, "user.store.FindByID")
	})

	t.Run("FindByID returns not found for missing or deleted users", func(t *testing.T) {
		store := resetStore(t, ctx, client)
		missingID := bson.NewObjectID().Hex()
		got, err := store.FindByID(ctx, missingID)
		if got != nil {
			t.Fatalf("missing user: got %#v, want nil", got)
		}
		requireAppError(t, err, apperror.ErrorVariantNotFound, "user.store.FindByID")

		deletedID, err := store.Insert(ctx, validUser("Grace", "Hopper", "grace@example.com"))
		if err != nil {
			t.Fatalf("insert deleted user: %v", err)
		}
		if err := store.SoftDeleteByID(ctx, deletedID); err != nil {
			t.Fatalf("soft delete user: %v", err)
		}
		got, err = store.FindByID(ctx, deletedID)
		if got != nil {
			t.Fatalf("deleted user: got %#v, want nil", got)
		}
		requireAppError(t, err, apperror.ErrorVariantNotFound, "user.store.FindByID")
	})

	t.Run("FindByID returns decode errors", func(t *testing.T) {
		store := resetStore(t, ctx, client)
		result, err := collection(t, client).InsertOne(ctx, bson.M{
			"first_name": "Bad",
			"last_name":  "Data",
			"email":      "bad@example.com",
			"password":   "hashed",
			"created_at": "not-a-time",
			"updated_at": time.Now().UTC(),
		})
		if err != nil {
			t.Fatalf("insert malformed user: %v", err)
		}

		got, err := store.FindByID(ctx, result.InsertedID.(bson.ObjectID).Hex())
		if got != nil {
			t.Fatalf("user: got %#v, want nil", got)
		}
		requireAppError(t, err, apperror.ErrorVariantInternal, "user.store.FindByID")
	})

	t.Run("Insert returns internal errors", func(t *testing.T) {
		store := disconnectedStore(t, ctx, uri)

		got, err := store.Insert(ctx, validUser("Ada", "Lovelace", "ada@example.com"))
		if got != "" {
			t.Fatalf("id: got %q, want empty", got)
		}
		requireAppError(t, err, apperror.ErrorVariantInternal, "user.store.Insert")
	})

	t.Run("UpdateByID updates names and timestamp only", func(t *testing.T) {
		store := resetStore(t, ctx, client)
		id, err := store.Insert(ctx, validUser("Ada", "Lovelace", "ada@example.com"))
		if err != nil {
			t.Fatalf("insert user: %v", err)
		}
		updatedAt := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)

		err = store.UpdateByID(ctx, id, &user{
			FirstName: "Augusta",
			LastName:  "Byron",
			Email:     "ignored@example.com",
			Password:  "ignored",
			UpdatedAt: updatedAt,
		})
		if err != nil {
			t.Fatalf("UpdateByID: %v", err)
		}

		got, err := store.FindByID(ctx, id)
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if got.FirstName != "Augusta" || got.LastName != "Byron" {
			t.Fatalf("name: got %q %q", got.FirstName, got.LastName)
		}
		if got.Email != "ada@example.com" || got.Password != "hashed-password" {
			t.Fatalf("immutable fields changed: got email=%q password=%q", got.Email, got.Password)
		}
		if !got.UpdatedAt.Equal(updatedAt) {
			t.Fatalf("updated_at: got %v, want %v", got.UpdatedAt, updatedAt)
		}
	})

	t.Run("UpdateByID returns not found for invalid, missing, or deleted users", func(t *testing.T) {
		store := resetStore(t, ctx, client)
		err := store.UpdateByID(ctx, "bad-id", validUser("Ada", "Lovelace", "ada@example.com"))
		requireAppError(t, err, apperror.ErrorVariantNotFound, "user.store.Update")

		err = store.UpdateByID(ctx, bson.NewObjectID().Hex(), validUser("Ada", "Lovelace", "ada@example.com"))
		requireAppError(t, err, apperror.ErrorVariantNotFound, "user.store.Update")

		deletedID, err := store.Insert(ctx, validUser("Grace", "Hopper", "grace@example.com"))
		if err != nil {
			t.Fatalf("insert deleted user: %v", err)
		}
		if err := store.SoftDeleteByID(ctx, deletedID); err != nil {
			t.Fatalf("soft delete user: %v", err)
		}
		err = store.UpdateByID(ctx, deletedID, validUser("Grace", "Murray", "grace@example.com"))
		requireAppError(t, err, apperror.ErrorVariantNotFound, "user.store.Update")
	})

	t.Run("UpdateByID returns internal errors", func(t *testing.T) {
		store := disconnectedStore(t, ctx, uri)

		err := store.UpdateByID(ctx, bson.NewObjectID().Hex(), validUser("Ada", "Lovelace", "ada@example.com"))
		requireAppError(t, err, apperror.ErrorVariantInternal, "user.store.Update")
	})

	t.Run("SoftDeleteByID hides user and stamps deletion", func(t *testing.T) {
		store := resetStore(t, ctx, client)
		id, err := store.Insert(ctx, validUser("Ada", "Lovelace", "ada@example.com"))
		if err != nil {
			t.Fatalf("insert user: %v", err)
		}

		before := time.Now().UTC()
		if err := store.SoftDeleteByID(ctx, id); err != nil {
			t.Fatalf("SoftDeleteByID: %v", err)
		}
		after := time.Now().UTC()

		got, err := store.FindByID(ctx, id)
		if got != nil {
			t.Fatalf("deleted user: got %#v, want nil", got)
		}
		requireAppError(t, err, apperror.ErrorVariantNotFound, "user.store.FindByID")

		var raw user
		objectID, err := bson.ObjectIDFromHex(id)
		if err != nil {
			t.Fatalf("object id from hex: %v", err)
		}
		if err := collection(t, client).FindOne(ctx, bson.M{"_id": objectID}).Decode(&raw); err != nil {
			t.Fatalf("find raw deleted user: %v", err)
		}
		if raw.DeletedAt == nil {
			t.Fatal("deleted_at: got nil")
		}
		if raw.DeletedAt.Before(before.Add(-time.Second)) || raw.DeletedAt.After(after.Add(time.Second)) {
			t.Fatalf("deleted_at %v outside [%v, %v]", raw.DeletedAt, before, after)
		}
		if !raw.UpdatedAt.Equal(*raw.DeletedAt) {
			t.Fatalf("updated_at: got %v, want %v", raw.UpdatedAt, *raw.DeletedAt)
		}
	})

	t.Run("SoftDeleteByID returns not found for invalid, missing, or deleted users", func(t *testing.T) {
		store := resetStore(t, ctx, client)
		err := store.SoftDeleteByID(ctx, "bad-id")
		requireAppError(t, err, apperror.ErrorVariantNotFound, "user.store.SoftDelete")

		err = store.SoftDeleteByID(ctx, bson.NewObjectID().Hex())
		requireAppError(t, err, apperror.ErrorVariantNotFound, "user.store.SoftDelete")

		deletedID, err := store.Insert(ctx, validUser("Grace", "Hopper", "grace@example.com"))
		if err != nil {
			t.Fatalf("insert deleted user: %v", err)
		}
		if err := store.SoftDeleteByID(ctx, deletedID); err != nil {
			t.Fatalf("soft delete user: %v", err)
		}
		err = store.SoftDeleteByID(ctx, deletedID)
		requireAppError(t, err, apperror.ErrorVariantNotFound, "user.store.SoftDelete")
	})

	t.Run("SoftDeleteByID returns internal errors", func(t *testing.T) {
		store := disconnectedStore(t, ctx, uri)

		err := store.SoftDeleteByID(ctx, bson.NewObjectID().Hex())
		requireAppError(t, err, apperror.ErrorVariantInternal, "user.store.SoftDelete")
	})
}

func newMongoClient(t *testing.T, ctx context.Context, uri string) *mongo.Client {
	t.Helper()

	client := mustConnectClient(t, options.Client().
		ApplyURI(uri).
		SetBSONOptions(&options.BSONOptions{ObjectIDAsHexString: true}))
	if err := client.Ping(ctx, nil); err != nil {
		t.Fatalf("ping mongo: %v", err)
	}
	return client
}

func mustConnectClient(t *testing.T, opts *options.ClientOptions) *mongo.Client {
	t.Helper()

	client, err := mongo.Connect(opts)
	if err != nil {
		t.Fatalf("connect mongo: %v", err)
	}
	return client
}

func resetStore(t *testing.T, ctx context.Context, client *mongo.Client) *mongoStore {
	t.Helper()

	if err := collection(t, client).Drop(ctx); err != nil {
		t.Fatalf("drop users collection: %v", err)
	}
	got, err := newMongoStore(client)
	if err != nil {
		t.Fatalf("new mongo store: %v", err)
	}
	store, ok := got.(*mongoStore)
	if !ok {
		t.Fatalf("store type: got %T, want *mongoStore", got)
	}
	return store
}

func disconnectedStore(t *testing.T, ctx context.Context, uri string) *mongoStore {
	t.Helper()

	client := newMongoClient(t, ctx, uri)
	got, err := newMongoStore(client)
	if err != nil {
		t.Fatalf("new mongo store: %v", err)
	}
	if err := client.Disconnect(ctx); err != nil {
		t.Fatalf("disconnect mongo client: %v", err)
	}
	return got.(*mongoStore)
}

func collection(t *testing.T, client *mongo.Client) *mongo.Collection {
	t.Helper()

	return client.Database(databaseName).Collection(collectionName)
}

func validUser(firstName, lastName, email string) *user {
	now := time.Now().UTC()
	return &user{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
		Password:  "hashed-password",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func requireAppError(t *testing.T, err error, variant apperror.ErrorVariant, op string) {
	t.Helper()

	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected app error, got %T: %v", err, err)
	}
	if appErr.Variant != variant {
		t.Fatalf("variant: got %q, want %q", appErr.Variant, variant)
	}
	if appErr.Op != op {
		t.Fatalf("op: got %q, want %q", appErr.Op, op)
	}
}

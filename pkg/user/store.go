package user

import (
	"context"
	"errors"
	"time"

	"github.com/lutia-io/huma/pkg/apperror"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	databaseName   = "huma"
	collectionName = "users"
)

type store interface {
	Find(ctx context.Context) ([]user, error)
	FindByID(ctx context.Context, id string) (*user, error)
	Insert(ctx context.Context, user *user) (string, error)
	UpdateByID(ctx context.Context, id string, user *user) error
	SoftDeleteByID(ctx context.Context, id string) error
}

type mongoStore struct {
	client *mongo.Client
}

func newMongoStore(client *mongo.Client) (store, error) {
	store := &mongoStore{client: client}
	if err := registerIndexes(context.Background(), client); err != nil {
		return nil, err
	}
	return store, nil
}

func registerIndexes(ctx context.Context, client *mongo.Client) error {
	_, err := client.Database(databaseName).Collection(collectionName).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

func (store *mongoStore) Find(ctx context.Context) ([]user, error) {
	filter := bson.M{"deleted_at": nil}
	cursor, err := store.client.Database(databaseName).Collection(collectionName).Find(ctx, filter)
	if err != nil {
		return nil, apperror.NewInternalError("user.store.Find", "Failed to query users", err)
	}
	defer cursor.Close(ctx)

	users := make([]user, 0)
	if err := cursor.All(ctx, &users); err != nil {
		return nil, apperror.NewInternalError("user.store.Find", "Failed to decode users", err)
	}
	return users, nil
}

func (store *mongoStore) FindByID(ctx context.Context, id string) (*user, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apperror.NewNotFoundError("user.store.FindByID", "User not found", err)
	}

	filter := bson.M{"_id": objectID, "deleted_at": nil}
	var user user
	if err := store.client.Database(databaseName).Collection(collectionName).FindOne(ctx, filter).Decode(&user); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, apperror.NewNotFoundError("user.store.FindByID", "User not found", err)
		}
		return nil, apperror.NewInternalError("user.store.FindByID", "Failed to find user", err)
	}
	return &user, nil
}

func (store *mongoStore) Insert(ctx context.Context, user *user) (string, error) {
	result, err := store.client.Database(databaseName).Collection(collectionName).InsertOne(ctx, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return "", apperror.NewConflictError("user.store.Insert", "User already exists", err)
		}
		return "", apperror.NewInternalError("user.store.Insert", "Failed to insert user", err)
	}

	user.ID = result.InsertedID.(bson.ObjectID).Hex()
	return user.ID, nil
}

func (store *mongoStore) UpdateByID(ctx context.Context, id string, user *user) error {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apperror.NewNotFoundError("user.store.Update", "User not found", err)
	}

	filter := bson.M{"_id": objectID, "deleted_at": nil}
	update := bson.M{
		"$set": bson.M{
			"first_name": user.FirstName,
			"last_name":  user.LastName,
			"updated_at": user.UpdatedAt,
		},
	}
	result, err := store.client.Database(databaseName).Collection(collectionName).UpdateOne(ctx, filter, update)
	if err != nil {
		return apperror.NewInternalError("user.store.Update", "Failed to update user", err)
	}
	if result.MatchedCount == 0 {
		return apperror.NewNotFoundError("user.store.Update", "User not found", nil)
	}
	return nil
}

func (store *mongoStore) SoftDeleteByID(ctx context.Context, id string) error {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apperror.NewNotFoundError("user.store.SoftDelete", "User not found", err)
	}
	now := time.Now().UTC()
	filter := bson.M{"_id": objectID, "deleted_at": nil}
	update := bson.M{
		"$set": bson.M{
			"deleted_at": now,
			"updated_at": now,
		},
	}
	result, err := store.client.Database(databaseName).Collection(collectionName).UpdateOne(ctx, filter, update)
	if err != nil {
		return apperror.NewInternalError("user.store.SoftDelete", "Failed to delete user", err)
	}
	if result.MatchedCount == 0 {
		return apperror.NewNotFoundError("user.store.SoftDelete", "User not found", nil)
	}
	return nil
}

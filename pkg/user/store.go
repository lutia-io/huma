package user

import (
	"context"

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
	FindById(ctx context.Context, id string) (*user, error)
	Insert(ctx context.Context, user *user) (string, error)
}

type mongoStore struct {
	client *mongo.Client
}

func newMongoStore(client *mongo.Client) store {
	store := &mongoStore{client: client}
	if err := registerIndexes(context.Background(), client); err != nil {
		panic(err)
	}
	return store
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

func (store *mongoStore) FindById(ctx context.Context, id string) (*user, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apperror.NewNotFoundError("user.store.FindById", "User not found", err)
	}

	filter := bson.M{"_id": objectID, "deleted_at": nil}
	var user user
	if err := store.client.Database(databaseName).Collection(collectionName).FindOne(ctx, filter).Decode(&user); err != nil {
		return nil, apperror.NewNotFoundError("user.store.FindById", "User not found", err)
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

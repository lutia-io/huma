package network

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
	collectionName = "networks"
)

type store interface {
	Find(ctx context.Context) ([]network, error)
	FindByID(ctx context.Context, id string) (*network, error)
	Insert(ctx context.Context, network *network) (string, error)
	UpdateByID(ctx context.Context, id string, network *network) error
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
		Keys:    bson.D{{Key: "slug", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return err
}

func (store *mongoStore) Find(ctx context.Context) ([]network, error) {
	filter := bson.M{"deleted_at": nil}
	cursor, err := store.client.Database(databaseName).Collection(collectionName).Find(ctx, filter)
	if err != nil {
		return nil, apperror.NewInternalError("network.store.Find", "Failed to query networks", err)
	}
	defer cursor.Close(ctx)

	networks := make([]network, 0)
	if err := cursor.All(ctx, &networks); err != nil {
		return nil, apperror.NewInternalError("network.store.Find", "Failed to decode networks", err)
	}
	return networks, nil
}

func (store *mongoStore) FindByID(ctx context.Context, id string) (*network, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, apperror.NewNotFoundError("network.store.FindByID", "Network not found", err)
	}

	filter := bson.M{"_id": objectID, "deleted_at": nil}
	var network network
	if err := store.client.Database(databaseName).Collection(collectionName).FindOne(ctx, filter).Decode(&network); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, apperror.NewNotFoundError("network.store.FindByID", "Network not found", err)
		}
		return nil, apperror.NewInternalError("network.store.FindByID", "Failed to find network", err)
	}
	return &network, nil
}

func (store *mongoStore) Insert(ctx context.Context, network *network) (string, error) {
	result, err := store.client.Database(databaseName).Collection(collectionName).InsertOne(ctx, network)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return "", apperror.NewConflictError("network.store.Insert", "Network already exists", err)
		}
		return "", apperror.NewInternalError("network.store.Insert", "Failed to insert network", err)
	}

	network.ID = result.InsertedID.(bson.ObjectID).Hex()
	return network.ID, nil
}

func (store *mongoStore) UpdateByID(ctx context.Context, id string, network *network) error {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apperror.NewNotFoundError("network.store.Update", "Network not found", err)
	}

	filter := bson.M{"_id": objectID, "deleted_at": nil}
	update := bson.M{
		"$set": bson.M{
			"name":       network.Name,
			"updated_at": network.UpdatedAt,
		},
	}
	result, err := store.client.Database(databaseName).Collection(collectionName).UpdateOne(ctx, filter, update)
	if err != nil {
		return apperror.NewInternalError("network.store.Update", "Failed to update network", err)
	}
	if result.MatchedCount == 0 {
		return apperror.NewNotFoundError("network.store.Update", "Network not found", nil)
	}
	return nil
}

func (store *mongoStore) SoftDeleteByID(ctx context.Context, id string) error {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apperror.NewNotFoundError("network.store.SoftDelete", "Network not found", err)
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
		return apperror.NewInternalError("network.store.SoftDelete", "Failed to delete network", err)
	}
	if result.MatchedCount == 0 {
		return apperror.NewNotFoundError("network.store.SoftDelete", "Network not found", nil)
	}
	return nil
}

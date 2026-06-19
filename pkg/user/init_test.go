package user

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lutia-io/huma/pkg/logger"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestNew(t *testing.T) {
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

	mux := http.NewServeMux()
	New(logger.NewWithWriter(io.Discard), client, mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/users", nil))

	requireStatus(t, rec, http.StatusOK)
}

func TestNew_panicsWhenStoreCannotInitialize(t *testing.T) {
	badClient, err := mongo.Connect(options.Client().
		ApplyURI("mongodb://127.0.0.1:1").
		SetServerSelectionTimeout(10 * time.Millisecond).
		SetBSONOptions(&options.BSONOptions{ObjectIDAsHexString: true}))
	if err != nil {
		t.Fatalf("connect bad mongo client: %v", err)
	}
	t.Cleanup(func() { _ = badClient.Disconnect(context.Background()) })

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()

	New(logger.NewWithWriter(io.Discard), badClient, http.NewServeMux())
}

package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/lutia-io/huma/pkg/middleware"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

func New() {
	logger := slog.Default()

	uri := os.Getenv("API_SERVICE_MONGO_URI")
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		logger.Error("failed to connect to mongo", "error", err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			logger.Error("failed to disconnect from mongo", "error", err)
		}
	}()

	if err := client.Ping(context.Background(), readpref.Primary()); err != nil {
		logger.Error("failed to ping mongo", "error", err)
	}

	mux := http.NewServeMux()
	handler := http.Handler(mux)

	handler = middleware.NewTrailingSlashRedirect(handler)
	handler = middleware.NewBodySizeLimit(5<<20, handler)
	handler = middleware.NewTimeout(30*time.Second, handler)
	handler = middleware.NewRecover(logger, handler)
	handler = middleware.NewLogger(logger, handler)
	handler = middleware.NewRequestID(handler)
	handler = middleware.NewRealIP(handler)

	port := os.Getenv("API_SERVICE_PORT")
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	logger.Info("API is listening and serving", "port", port)
	if err := srv.ListenAndServe(); err != nil {
		logger.Error("Failed to listen and serve", "error", err)
	}
}

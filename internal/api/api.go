package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/middleware"
	"github.com/lutia-io/huma/pkg/user"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

func New() {
	log := logger.New()

	uri := os.Getenv("API_SERVICE_MONGO_URI")
	client, err := mongo.Connect(options.Client().
		ApplyURI(uri).
		SetBSONOptions(&options.BSONOptions{ObjectIDAsHexString: true}))
	if err != nil {
		log.Error("failed to connect to mongo", logger.KeyError, err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			log.Error("failed to disconnect from mongo", logger.KeyError, err)
		}
	}()

	if err := client.Ping(context.Background(), readpref.Primary()); err != nil {
		log.Error("failed to ping mongo", logger.KeyError, err)
	}

	mux := http.NewServeMux()
	handler := http.Handler(mux)

	user.New(log, client, mux)

	handler = middleware.NewTrailingSlashRedirect(handler)
	handler = middleware.NewBodySizeLimit(5<<20, handler)
	handler = middleware.NewTimeout(30*time.Second, handler)
	handler = middleware.NewRecover(log, handler)
	handler = middleware.NewLogger(log, handler)
	handler = middleware.NewRequestID(handler)
	handler = middleware.NewRealIP(handler)

	port := os.Getenv("API_SERVICE_PORT")
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Info("API is listening and serving", "port", port)
	if err := srv.ListenAndServe(); err != nil {
		log.Error("Failed to listen and serve", logger.KeyError, err)
	}
}

package user

import (
	"log/slog"
	"net/http"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

func Init(logger *slog.Logger, client *mongo.Client, mux *http.ServeMux) {
	store := NewMongoStore(client)
	service := NewService(logger, store)
	newHTTPHandler(service, mux)
}

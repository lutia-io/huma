package user

import (
	"net/http"

	"github.com/lutia-io/huma/pkg/logger"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func New(logger *logger.Logger, client *mongo.Client, mux *http.ServeMux) {
	store := newMongoStore(client)
	service := newService(logger, store)
	newHTTPHandler(service, mux)
}

package user

import (
	"net/http"

	"github.com/lutia-io/huma/pkg/hasher"
	"github.com/lutia-io/huma/pkg/logger"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func New(logger *logger.Logger, client *mongo.Client, mux *http.ServeMux) {
	store, err := newMongoStore(client)
	if err != nil {
		panic(err)
	}
	service := newService(logger, store, hasher.NewArgon2IDHasher())
	newHTTPHandler(service, mux)
}

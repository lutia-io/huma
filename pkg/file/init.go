package file

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/nats-io/nats.go/jetstream"
)

func New(logger *logger.Logger, pool *pgxpool.Pool, mux *http.ServeMux, objs jetstream.ObjectStore) *Service {
	service := NewWithPool(logger, pool, objs)
	newHTTPHandler(service, mux)
	return service
}

// NewWithPool constructs a Service without registering HTTP handlers, for
// consumers like the workflow engine or pipeline nodes.
func NewWithPool(logger *logger.Logger, pool *pgxpool.Pool, objs jetstream.ObjectStore) *Service {
	return NewService(logger, newPostgresStore(pool), objs)
}

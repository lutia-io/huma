package record

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/organizationuser"
	"github.com/lutia-io/huma/pkg/schema"
	"github.com/nats-io/nats.go/jetstream"
)

func New(logger *logger.Logger, pool *pgxpool.Pool, mux *http.ServeMux, js jetstream.JetStream, schemaService *schema.Service, orgUsers *organizationuser.Service) *Service {
	service := NewWithPool(logger, pool, js, schemaService)
	newHTTPHandler(service, orgUsers, mux)
	return service
}

// NewWithPool constructs a Service without registering HTTP handlers, for
// consumers like the workflow engine.
func NewWithPool(logger *logger.Logger, pool *pgxpool.Pool, js jetstream.JetStream, schemaService *schema.Service) *Service {
	return NewService(logger, newPostgresStore(pool), js, schemaService)
}

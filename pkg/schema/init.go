package schema

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
)

func New(logger *logger.Logger, pool *pgxpool.Pool, mux *http.ServeMux) *Service {
	store := newPostgresStore(pool)
	service := NewService(logger, store)
	newHTTPHandler(service, mux)
	return service
}

// NewWithPool constructs a Service without registering HTTP handlers, for
// consumers like the workflow engine that only need validation and resolution.
func NewWithPool(logger *logger.Logger, pool *pgxpool.Pool) *Service {
	return NewService(logger, newPostgresStore(pool))
}

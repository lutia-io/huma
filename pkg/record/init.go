package record

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/schema"
)

func New(logger *logger.Logger, pool *pgxpool.Pool, mux *http.ServeMux, schemaService *schema.Service) {
	store := newPostgresStore(pool)
	service := NewService(logger, store, schemaService)
	newHTTPHandler(service, mux)
}

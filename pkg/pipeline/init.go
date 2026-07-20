package pipeline

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

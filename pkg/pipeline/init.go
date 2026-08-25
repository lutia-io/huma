package pipeline

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/node"
)

func New(logger *logger.Logger, pool *pgxpool.Pool, mux *http.ServeMux, nodes *node.Service) *Service {
	store := NewPostgresStore(pool)
	service := NewService(logger, store, nodes)
	newHTTPHandler(service, mux)
	return service
}

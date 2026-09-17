package pipeline

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/authz"
	"github.com/lutia-io/huma/pkg/logger"
)

func New(logger *logger.Logger, pool *pgxpool.Pool, mux *http.ServeMux, engine *authz.Engine) *Service {
	store := NewPostgresStore(pool)
	service := NewService(logger, store, engine)
	newHTTPHandler(service, mux)
	return service
}

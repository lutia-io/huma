package organizationuser

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/hasher"
	"github.com/lutia-io/huma/pkg/logger"
)

func New(logger *logger.Logger, pool *pgxpool.Pool, mux *http.ServeMux) *Service {
	service := NewWithPool(logger, pool)
	newHTTPHandler(service, mux)
	return service
}

func NewWithPool(logger *logger.Logger, pool *pgxpool.Pool) *Service {
	return NewService(
		logger,
		newPostgresStore(pool),
		hasher.NewArgon2IDHasher(),
	)
}

package organizationuser

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/authz"
	"github.com/lutia-io/huma/pkg/hasher"
	"github.com/lutia-io/huma/pkg/logger"
)

func New(logger *logger.Logger, pool *pgxpool.Pool, mux *http.ServeMux, engine *authz.Engine) *Service {
	service := NewWithPool(logger, pool, engine)
	newHTTPHandler(service, mux)
	return service
}

func NewWithPool(logger *logger.Logger, pool *pgxpool.Pool, engine *authz.Engine) *Service {
	if engine == nil {
		engine = authz.New(pool)
	}
	return NewService(
		logger,
		newPostgresStore(pool),
		hasher.NewArgon2IDHasher(),
		engine,
	)
}

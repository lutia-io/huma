package networkaccess

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/authz"
	"github.com/lutia-io/huma/pkg/logger"
)

func New(logger *logger.Logger, pool *pgxpool.Pool, mux *http.ServeMux, engine *authz.Engine) {
	store := newPostgresStore(pool)
	service := newService(logger, store, engine)
	newHTTPHandler(service, mux)
}

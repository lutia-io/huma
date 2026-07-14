package record

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/schema"
	"github.com/lutia-io/huma/pkg/workflow"
)

func New(logger *logger.Logger, pool *pgxpool.Pool, mux *http.ServeMux, schemaService *schema.Service, workflowService *workflow.Service) {
	store := newPostgresStore(pool)
	service := NewService(logger, store, schemaService, workflowService)
	newHTTPHandler(service, mux)
}

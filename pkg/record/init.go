package record

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/schema"
	"github.com/lutia-io/huma/pkg/workflow"
	"github.com/nats-io/nats.go/jetstream"
)

func New(logger *logger.Logger, pool *pgxpool.Pool, mux *http.ServeMux, js jetstream.JetStream, schemaService *schema.Service, workflowService *workflow.Service) {
	store := newPostgresStore(pool)
	service := NewService(logger, store, js, schemaService, workflowService)
	newHTTPHandler(service, mux)
}

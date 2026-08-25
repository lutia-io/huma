package workflow

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/node"
	"github.com/lutia-io/huma/pkg/pipeline"
	"github.com/lutia-io/huma/pkg/record"
	"github.com/lutia-io/huma/pkg/schema"
	"github.com/lutia-io/huma/pkg/workflow/executor"
	"github.com/lutia-io/huma/pkg/workflow/executor/handlers"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	defaultWorkerCount = 4
	workerPollInterval = time.Second

	// workerLeaseTimeout bounds how long a crashed worker's workflow stays
	// unclaimable. Workers heartbeat the lease after every action, so only a
	// single action running longer than this can be claimed twice — which the
	// action idempotency keys absorb.
	workerLeaseTimeout = 2 * time.Minute
)

// NewExecutor runs the workflow executor service: a worker pool that claims
// pending workflows from Postgres and executes their actions. It consumes no
// events; work arrives as rows inserted by the evaluator service. It scales
// horizontally, with workers coordinating through the claim query's row locks.
func NewExecutor() {
	log := logger.New()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, os.Getenv("HUMA_SERVICE_POSTGRES_RW_URI"))
	if err != nil {
		log.Error("Unable to create db connection pool", logger.KeyError, err)
		os.Exit(1)
	}
	defer pool.Close()

	// NATS is not used for claiming work, but action handlers publish events
	// (e.g. records created by workflows) through JetStream.
	nc, err := nats.Connect(os.Getenv("HUMA_SERVICE_NATS_URI"))
	if err != nil {
		log.Error("Unable to create NATS connection", logger.KeyError, err)
		os.Exit(1)
	}
	defer nc.Drain()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Error("Unable to create JetStream", logger.KeyError, err)
		os.Exit(1)
	}

	workflowStore := executor.NewPostgresWorkflowStore(pool, workerLeaseTimeout)
	schemaService := schema.NewWithPool(log, pool)
	recordService := record.NewWithPool(log, pool, js, schemaService)
	nodeService := node.NewService(log, node.NewPostgresStore(pool))
	pipelineService := pipeline.NewService(log, pipeline.NewPostgresStore(pool), nodeService)

	registry := executor.NewRegistry(
		handlers.NewCreateRecord(recordService),
		handlers.NewUpdateRecord(recordService, schemaService),
		handlers.NewUpsertRecord(recordService, schemaService),
		handlers.NewTriggerPipeline(log, pipelineService),
	)

	service := executor.NewService(log, workflowStore, registry)
	service.StartWorkers(ctx, workerCount(log), "workflow-executor", workerPollInterval)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	port := os.Getenv("HUMA_SERVICE_PORT")
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Info("Workflow executor is listening and serving", logger.KeyPort, port)
	if err := srv.ListenAndServe(); err != nil {
		log.Error("Failed to listen and serve", logger.KeyError, err)
	}
}

func workerCount(log *logger.Logger) int {
	raw := os.Getenv("HUMA_WORKFLOW_WORKER_COUNT")
	if raw == "" {
		return defaultWorkerCount
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		log.WarnContext(context.Background(), "Invalid HUMA_WORKFLOW_WORKER_COUNT, using default", "value", raw, logger.KeyCount, defaultWorkerCount)
		return defaultWorkerCount
	}
	return n
}

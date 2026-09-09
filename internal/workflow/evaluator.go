// Package workflow hosts the two workflow engine services.
// The evaluator is the intake side: it consumes record events and inserts pending workflows.
// The executor is the execution side: a worker pool that claims and runs
// them. They share nothing at runtime but the workflows table, so each
// deploys and scales independently.
package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/metrics"
	"github.com/lutia-io/huma/pkg/record"
	wf "github.com/lutia-io/huma/pkg/workflow"
	"github.com/lutia-io/huma/pkg/workflow/executor"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NewEvaluator runs the workflow evaluator service: it consumes record
// created and updated events, ticks scheduled definitions once a minute, and
// durably inserts one pending workflow per match. No action side effects
// happen here; after the ack, Postgres is the source of truth for the
// workflow and the executor service picks it up.
func NewEvaluator() {
	log := logger.New()
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, os.Getenv("HUMA_SERVICE_POSTGRES_RW_URI"))
	if err != nil {
		log.Error("Unable to create db connection pool", logger.KeyError, err)
		os.Exit(1)
	}
	defer pool.Close()

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

	definitionStore := wf.NewPostgresStore(pool)
	workflowStore := executor.NewPostgresWorkflowStore(pool, workerLeaseTimeout)
	enqueuer := executor.NewEnqueuer(log, definitionStore, workflowStore)
	recordPager := record.NewSchemaPager(pool)

	consumer, err := js.CreateOrUpdateConsumer(ctx, record.StreamName, jetstream.ConsumerConfig{
		Durable:        "workflow-evaluator",
		FilterSubjects: []string{record.SubjectCreated, record.SubjectUpdated},
		AckPolicy:      jetstream.AckExplicitPolicy,
	})
	if err != nil {
		log.Error("Unable to create consumer", logger.KeyError, err)
		os.Exit(1)
	}

	_, err = consumer.Consume(func(msg jetstream.Msg) {
		msgCtx := context.Background()
		if err := evaluateMessage(msgCtx, log, enqueuer, msg); err != nil {
			if errors.Is(err, errMessageSettled) {
				return
			}
			log.Error("Failed to evaluate workflows for record", logger.KeyError, err)
			if nakErr := msg.Nak(); nakErr != nil {
				log.Error("Failed to nack message", logger.KeyError, nakErr)
			}
			return
		}
		if err := msg.Ack(); err != nil {
			log.Error("Failed to ack message", logger.KeyError, err)
		}
	})
	if err != nil {
		log.Error("Unable to start consumer", logger.KeyError, err)
		os.Exit(1)
	}

	go runScheduleTicker(ctx, log, pool, definitionStore, recordPager, enqueuer)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.Handle("GET /metrics", metrics.Handler())

	port := os.Getenv("HUMA_SERVICE_PORT")
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Info("Workflow evaluator is listening and serving", logger.KeyPort, port)
	if err := srv.ListenAndServe(); err != nil {
		log.Error("Failed to listen and serve", logger.KeyError, err)
	}
}

var errMessageSettled = errors.New("message settled")

func evaluateMessage(ctx context.Context, log *logger.Logger, enqueuer *executor.Enqueuer, msg jetstream.Msg) error {
	switch msg.Subject() {
	case record.SubjectCreated:
		var event record.CreatedEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			log.Error("Failed to unmarshal record created event", logger.KeyError, err)
			if termErr := msg.Term(); termErr != nil {
				log.Error("Failed to terminate message", logger.KeyError, termErr)
			}
			return errMessageSettled
		}
		return enqueuer.EvaluateCreated(ctx, event)
	case record.SubjectUpdated:
		var event record.UpdatedEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			log.Error("Failed to unmarshal record updated event", logger.KeyError, err)
			if termErr := msg.Term(); termErr != nil {
				log.Error("Failed to terminate message", logger.KeyError, termErr)
			}
			return errMessageSettled
		}
		return enqueuer.EvaluateUpdated(ctx, event)
	default:
		log.Error("Unknown record event subject", "subject", msg.Subject())
		if termErr := msg.Term(); termErr != nil {
			log.Error("Failed to terminate message", logger.KeyError, termErr)
		}
		return errMessageSettled
	}
}

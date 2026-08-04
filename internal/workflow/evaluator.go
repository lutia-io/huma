// Package workflow hosts the two workflow engine services.
// The evaluator is the intake side: it consumes record events and inserts pending workflows.
// The executor is the execution side: a worker pool that claims and runs
// them. They share nothing at runtime but the workflows table, so each
// deploys and scales independently.
package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/record"
	wf "github.com/lutia-io/huma/pkg/workflow"
	"github.com/lutia-io/huma/pkg/workflow/executor"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NewEvaluator runs the workflow evaluator service: it consumes record
// events, evaluates workflow definition criteria against the record data, and
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

	consumer, err := js.CreateOrUpdateConsumer(ctx, record.StreamName, jetstream.ConsumerConfig{
		Durable:        "workflow-evaluator",
		FilterSubjects: []string{record.SubjectCreated},
		AckPolicy:      jetstream.AckExplicitPolicy,
	})
	if err != nil {
		log.Error("Unable to create consumer", logger.KeyError, err)
		os.Exit(1)
	}

	_, err = consumer.Consume(func(msg jetstream.Msg) {
		msgCtx := context.Background()

		var event record.CreatedEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			// A payload that cannot be parsed never will be; terminate it
			// instead of letting redelivery retry it forever.
			log.Error("Failed to unmarshal record created event", logger.KeyError, err)
			err = msg.Term()
			if err != nil {
				log.Error("Failed to terminate message", logger.KeyError, err)
			}
			return
		}

		if err := enqueuer.EvaluateRecord(msgCtx, event); err != nil {
			// Intake has no side effects; redelivery retries the enqueue and
			// the dedupe constraint absorbs any partial insert.
			log.Error("Failed to evaluate workflows for record", logger.KeyID, event.ID, logger.KeyError, err)
			err = msg.Nak()
			if err != nil {
				log.Error("Failed to nack message", logger.KeyError, err)
			}
			return
		}

		err = msg.Ack()
		if err != nil {
			log.Error("Failed to ack message", logger.KeyError, err)
		}
	})
	if err != nil {
		log.Error("Unable to start consumer", logger.KeyError, err)
		os.Exit(1)
	}

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
	log.Info("Workflow evaluator is listening and serving", logger.KeyPort, port)
	if err := srv.ListenAndServe(); err != nil {
		log.Error("Failed to listen and serve", logger.KeyError, err)
	}
}

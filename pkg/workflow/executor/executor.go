// Package executor is the workflow engine.
//
// It is built around durable execution: Postgres, not the message broker, is
// the source of truth for what work exists and what state it is in.
//
//   - The Enqueuer is the intake half. It evaluates workflow definitions
//     against a trigger event and inserts one pending workflow per match,
//     with the definition and record data snapshotted onto the workflow.
//   - The Service is the execution half. It runs a pool of workers that claim
//     workflows with FOR UPDATE SKIP LOCKED, execute their actions in order,
//     and journal every attempt. Leases make crashed workers' workflows
//     reclaimable; idempotency keys make replayed side effects converge
//     instead of duplicating.
//
// The two halves share nothing at runtime except the workflows table, so
// they deploy and scale as independent services.
package executor

import (
	"context"
	"time"

	"github.com/lutia-io/huma/pkg/logger"
)

// Service is the execution half of the engine: a worker pool over the
// workflows table. It does not consume events; work arrives as rows inserted
// by the Enqueuer.
type Service struct {
	logger    *logger.Logger
	workflows WorkflowStore
	registry  *Registry
}

func NewService(logger *logger.Logger, workflows WorkflowStore, registry *Registry) *Service {
	return &Service{
		logger:    logger,
		workflows: workflows,
		registry:  registry,
	}
}

// StartWorkers launches the worker pool and the exhausted-workflow sweeper.
// It returns immediately; workers run until ctx is cancelled. Workers
// coordinate only through the claim query's row locks, so the pool scales
// horizontally across processes and pods.
func (s *Service) StartWorkers(ctx context.Context, count int, workerIDPrefix string, pollInterval time.Duration) {
	for i := range count {
		w := &worker{
			id:           workerID(workerIDPrefix, i),
			service:      s,
			pollInterval: pollInterval,
		}
		go w.run(ctx)
	}
	go s.sweepExhausted(ctx)
	s.logger.Info("Workflow workers started", logger.KeyCount, count)
}

// sweepExhausted periodically fails workflows stuck in crash loops (attempts
// exhausted without reaching a terminal status). Without it, such workflows
// would sit unclaimable in a non-terminal status forever.
func (s *Service) sweepExhausted(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.workflows.FailExhausted(ctx)
			if err != nil {
				s.logger.ErrorContext(ctx, "Failed to sweep exhausted workflows", logger.KeyError, err)
				continue
			}
			if n > 0 {
				s.logger.WarnContext(ctx, "Marked exhausted workflows as failed", logger.KeyCount, n)
			}
		}
	}
}

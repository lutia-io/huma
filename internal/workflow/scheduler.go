package workflow

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lutia-io/huma/pkg/logger"
	"github.com/lutia-io/huma/pkg/record"
	wf "github.com/lutia-io/huma/pkg/workflow"
	"github.com/lutia-io/huma/pkg/workflow/executor"
)

// scheduleAdvisoryLockKey is a session-level lock so only one evaluator
// replica fans out scheduled workflows for a given tick.
const scheduleAdvisoryLockKey int64 = 0x68756d61776601

const schedulePageSize = 100

func runScheduleTicker(
	ctx context.Context,
	log *logger.Logger,
	pool *pgxpool.Pool,
	definitions executor.WorkflowDefinitionStore,
	records record.SchemaPager,
	enqueuer *executor.Enqueuer,
) {
	delay := time.Until(time.Now().Truncate(time.Minute).Add(time.Minute))
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		if !timer.Stop() {
			<-timer.C
		}
		return
	case <-timer.C:
		tickScheduledWorkflows(ctx, log, pool, definitions, records, enqueuer, time.Now())
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			tickScheduledWorkflows(ctx, log, pool, definitions, records, enqueuer, now)
		}
	}
}

func tickScheduledWorkflows(
	ctx context.Context,
	log *logger.Logger,
	pool *pgxpool.Pool,
	definitions executor.WorkflowDefinitionStore,
	records record.SchemaPager,
	enqueuer *executor.Enqueuer,
	now time.Time,
) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		log.ErrorContext(ctx, "Failed to acquire schedule lock connection", logger.KeyError, err)
		return
	}
	defer conn.Release()

	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", scheduleAdvisoryLockKey).Scan(&locked); err != nil {
		log.ErrorContext(ctx, "Failed to take schedule advisory lock", logger.KeyError, err)
		return
	}
	if !locked {
		return
	}
	defer func() {
		if _, err := conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", scheduleAdvisoryLockKey); err != nil {
			log.ErrorContext(ctx, "Failed to release schedule advisory lock", logger.KeyError, err)
		}
	}()

	defs, err := definitions.ListActiveScheduled(ctx)
	if err != nil {
		log.ErrorContext(ctx, "Failed to list scheduled workflow definitions", logger.KeyError, err)
		return
	}

	periodStart := now.UTC().Truncate(time.Minute)
	for _, def := range defs {
		due, err := wf.ScheduleDueAt(def.Definition.Trigger.Cron, def.Definition.Trigger.Timezone, now)
		if err != nil {
			log.ErrorContext(ctx, "Failed to evaluate schedule", logger.KeyID, def.ID, logger.KeyError, err)
			continue
		}
		if !due {
			continue
		}
		if err := fanoutScheduledDefinition(ctx, records, enqueuer, def, periodStart); err != nil {
			log.ErrorContext(ctx, "Failed to enqueue scheduled workflows", logger.KeyID, def.ID, logger.KeyError, err)
		}
	}
}

func fanoutScheduledDefinition(
	ctx context.Context,
	records record.SchemaPager,
	enqueuer *executor.Enqueuer,
	def *wf.WorkflowDefinition,
	periodStart time.Time,
) error {
	afterID := ""
	for {
		page, err := records.ListBySchema(ctx, def.NetworkID, def.SchemaID, afterID, schedulePageSize)
		if err != nil {
			return err
		}
		if len(page) == 0 {
			return nil
		}
		if err := enqueuer.EvaluateSchedule(ctx, def, page, periodStart); err != nil {
			return err
		}
		if len(page) < schedulePageSize {
			return nil
		}
		afterID = page[len(page)-1].ID
	}
}

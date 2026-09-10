-- workflows: one execution instance per (definition, trigger event).
-- The definition and trigger data are snapshotted at enqueue time so a run
-- always executes against the state it started with, regardless of later edits.
CREATE TABLE public.workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_definition_id UUID NOT NULL REFERENCES public.workflow_definitions(id),
    network_id UUID NOT NULL REFERENCES public.networks(id),
    -- The record that triggered the run; data is its content at trigger time
    -- and org identity flows from it onto records created by actions.
    record_id UUID NOT NULL REFERENCES public.records(id),
    data JSONB NOT NULL,
    organization_id UUID NOT NULL REFERENCES public.organizations(id),
    organization_user_id UUID NOT NULL REFERENCES public.organization_users(id),
    dedupe_key TEXT NOT NULL,
    definition JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    current_action INT NOT NULL DEFAULT 0,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    locked_by TEXT,
    locked_at TIMESTAMPTZ,
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    UNIQUE (workflow_definition_id, dedupe_key)
);

-- Partial index keeps the worker claim query fast regardless of how many
-- completed/failed rows accumulate.
CREATE INDEX workflows_claimable_idx
    ON public.workflows (next_attempt_at)
    WHERE status IN ('pending', 'running');

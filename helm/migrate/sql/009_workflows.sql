-- workflows: one execution instance per (definition, dedupe key).
-- The definition and input are snapshotted at enqueue time so a run
-- always executes against the state it started with, regardless of later edits.
CREATE TABLE public.workflows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_definition_id UUID NOT NULL REFERENCES public.workflow_definitions(id) ON DELETE CASCADE,
    network_id UUID NOT NULL REFERENCES public.networks(id) ON DELETE CASCADE,
    -- Optional trigger record; null for manual/API enqueue runs.
    record_id UUID REFERENCES public.records(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}',
    input JSONB NOT NULL DEFAULT '{}',
    organization_id UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    organization_user_id UUID NOT NULL REFERENCES public.organization_users(id) ON DELETE CASCADE,
    dedupe_key TEXT NOT NULL,
    definition JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    current_level INT NOT NULL DEFAULT 0,
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

CREATE INDEX workflows_claimable_idx
    ON public.workflows (next_attempt_at)
    WHERE status IN ('pending', 'running');

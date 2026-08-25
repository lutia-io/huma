-- pipelines: one execution instance per (definition, trigger).
-- The definition (resolved node snapshots) and input are frozen at enqueue
-- time so a run always executes against the state it started with.
CREATE TABLE public.pipelines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_definition_id UUID NOT NULL REFERENCES public.pipeline_definitions(id),
    network_id UUID NOT NULL REFERENCES public.networks(id),
    organization_id UUID NOT NULL REFERENCES public.organizations(id),
    organization_user_id UUID NOT NULL REFERENCES public.organization_users(id),
    dedupe_key TEXT NOT NULL,
    input JSONB NOT NULL,
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
    UNIQUE (pipeline_definition_id, dedupe_key)
);

CREATE INDEX pipelines_claimable_idx
    ON public.pipelines (next_attempt_at)
    WHERE status IN ('pending', 'running');

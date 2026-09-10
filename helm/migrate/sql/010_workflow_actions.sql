-- workflow_actions: append-only journal of action attempts. Rows are never
-- updated; failed attempts are preserved for audit and debugging.
CREATE TABLE public.workflow_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES public.workflows(id),
    action_index INT NOT NULL,
    attempt INT NOT NULL,
    action_type TEXT NOT NULL,
    status TEXT NOT NULL,
    input JSONB,
    output JSONB,
    error TEXT,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL,
    UNIQUE (workflow_id, action_index, attempt)
);

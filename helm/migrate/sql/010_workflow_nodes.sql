-- workflow_nodes: append-only journal of node attempts. Rows are never
-- updated; failed attempts are preserved for audit and debugging.
CREATE TABLE public.workflow_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES public.workflows(id) ON DELETE CASCADE,
    level_index INT NOT NULL,
    node_index INT NOT NULL,
    attempt INT NOT NULL,
    node_slug TEXT NOT NULL,
    node_type TEXT NOT NULL,
    status TEXT NOT NULL,
    input JSONB,
    output JSONB,
    payload JSONB,
    error TEXT,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL,
    UNIQUE (workflow_id, level_index, node_index, attempt)
);

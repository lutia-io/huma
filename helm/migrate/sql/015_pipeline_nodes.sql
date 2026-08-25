-- pipeline_nodes: append-only journal of node attempts. Rows are never
-- updated; failed attempts are preserved for audit and debugging.
CREATE TABLE public.pipeline_nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id UUID NOT NULL REFERENCES public.pipelines(id),
    level_index INT NOT NULL,
    node_index INT NOT NULL,
    attempt INT NOT NULL,
    node_definition_id UUID NOT NULL,
    node_slug TEXT NOT NULL,
    node_type TEXT NOT NULL,
    status TEXT NOT NULL,
    input JSONB,
    output JSONB,
    error TEXT,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL,
    UNIQUE (pipeline_id, level_index, node_index, attempt)
);

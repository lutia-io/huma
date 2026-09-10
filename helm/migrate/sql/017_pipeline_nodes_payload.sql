-- payload: files (and later other artifacts) produced by a node attempt.
-- Output JSON remains the next-level input; payload is journal/display only.
ALTER TABLE public.pipeline_nodes
    ADD COLUMN payload JSONB;

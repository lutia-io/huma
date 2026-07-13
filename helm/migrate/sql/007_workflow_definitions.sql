CREATE TABLE public.workflow_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    active BOOLEAN NOT NULL,
    internal BOOLEAN NOT NULL,
    definition JSONB NOT NULL,
    schema_id UUID NOT NULL REFERENCES public.schemas(id),
    network_id UUID NOT NULL REFERENCES public.networks(id),
    user_id UUID NOT NULL REFERENCES public.users(id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);
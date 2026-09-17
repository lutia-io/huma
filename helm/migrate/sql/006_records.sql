CREATE TABLE public.records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    data JSONB NOT NULL,
    schema_id UUID NOT NULL REFERENCES public.schemas(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    organization_user_id UUID NOT NULL REFERENCES public.organization_users(id) ON DELETE CASCADE,
    network_id UUID NOT NULL REFERENCES public.networks(id) ON DELETE CASCADE,
    idempotency_key TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX records_idempotency_key_idx
    ON public.records (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX records_network_schema_idx
    ON public.records (network_id, schema_id);

CREATE INDEX records_network_organization_schema_idx
    ON public.records (network_id, organization_id, schema_id);

CREATE TABLE public.records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    data JSONB NOT NULL,
    schema_id UUID NOT NULL REFERENCES public.schemas(id),
    organization_id UUID NOT NULL REFERENCES public.organizations(id),
    organization_user_id UUID NOT NULL REFERENCES public.organization_users(id),
    network_id UUID NOT NULL REFERENCES public.networks(id),
    idempotency_key TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX records_idempotency_key_idx
    ON public.records (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE TABLE public.files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    organization_id UUID NOT NULL REFERENCES public.organizations(id),
    organization_user_id UUID NOT NULL REFERENCES public.organization_users(id),
    network_id UUID NOT NULL REFERENCES public.networks(id),
    idempotency_key TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX files_idempotency_key_idx
    ON public.files (network_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX files_network_id_idx
    ON public.files (network_id)
    WHERE deleted_at IS NULL;

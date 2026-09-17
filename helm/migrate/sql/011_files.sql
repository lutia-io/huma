CREATE TABLE public.files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    organization_id UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    organization_user_id UUID NOT NULL REFERENCES public.organization_users(id) ON DELETE CASCADE,
    network_id UUID NOT NULL REFERENCES public.networks(id) ON DELETE CASCADE,
    idempotency_key TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX files_idempotency_key_idx
    ON public.files (network_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX files_network_id_idx
    ON public.files (network_id);

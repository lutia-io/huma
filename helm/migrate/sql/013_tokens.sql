CREATE TABLE public.tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash TEXT NOT NULL UNIQUE,
    family_id UUID NOT NULL,
    principal_type TEXT NOT NULL,
    principal_id UUID NOT NULL,
    network_id UUID REFERENCES public.networks(id),
    organization_id UUID REFERENCES public.organizations(id),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    replaced_by UUID REFERENCES public.tokens(id),
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX tokens_family_id_idx ON public.tokens (family_id);
CREATE INDEX tokens_principal_idx ON public.tokens (principal_type, principal_id);

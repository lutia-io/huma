CREATE TABLE public.organization_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    email TEXT NOT NULL,
    password TEXT NOT NULL,
    organization_id UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    network_id UUID NOT NULL REFERENCES public.networks(id) ON DELETE CASCADE,
    internal BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (network_id, email)
);

-- One hidden system user per organization, used as organization_user_id
-- on records created by workflow and pipeline nodes.
CREATE UNIQUE INDEX organization_users_internal_per_org_idx
    ON public.organization_users (organization_id)
    WHERE internal;

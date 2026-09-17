CREATE TABLE public.schemas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    internal BOOLEAN NOT NULL DEFAULT FALSE,
    -- JSON (not JSONB) preserves object key order for properties and TitleKey.
    definition JSON NOT NULL,
    network_id UUID NOT NULL REFERENCES public.networks(id) ON DELETE CASCADE,
    organization_id UUID REFERENCES public.organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES public.users(id),
    created_by UUID NOT NULL REFERENCES public.users(id),
    updated_by UUID NOT NULL REFERENCES public.users(id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    FOREIGN KEY (organization_id, network_id) REFERENCES public.organizations(id, network_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX schemas_network_slug_idx
    ON public.schemas (network_id, slug)
    WHERE organization_id IS NULL;

CREATE UNIQUE INDEX schemas_organization_slug_idx
    ON public.schemas (organization_id, slug)
    WHERE organization_id IS NOT NULL;

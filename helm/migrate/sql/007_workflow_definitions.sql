CREATE TABLE public.workflow_definitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL,
    internal BOOLEAN NOT NULL,
    definition JSONB NOT NULL,
    schema_id UUID NOT NULL REFERENCES public.schemas(id),
    network_id UUID NOT NULL REFERENCES public.networks(id),
    organization_id UUID REFERENCES public.organizations(id),
    user_id UUID NOT NULL REFERENCES public.users(id),
    created_by UUID NOT NULL REFERENCES public.users(id),
    updated_by UUID NOT NULL REFERENCES public.users(id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ,
    FOREIGN KEY (organization_id, network_id) REFERENCES public.organizations(id, network_id)
);

CREATE UNIQUE INDEX workflow_definitions_network_slug_idx
    ON public.workflow_definitions (network_id, slug)
    WHERE organization_id IS NULL;

CREATE UNIQUE INDEX workflow_definitions_organization_slug_idx
    ON public.workflow_definitions (organization_id, slug)
    WHERE organization_id IS NOT NULL;
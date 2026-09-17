CREATE TABLE public.organization_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    network_id UUID NOT NULL REFERENCES public.networks(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX organization_groups_org_slug_alive_idx
    ON public.organization_groups (organization_id, slug)
    WHERE deleted_at IS NULL;

CREATE TABLE public.organization_group_members (
    group_id UUID NOT NULL REFERENCES public.organization_groups(id) ON DELETE CASCADE,
    organization_user_id UUID NOT NULL REFERENCES public.organization_users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (group_id, organization_user_id)
);

CREATE TABLE public.organization_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES public.organizations(id) ON DELETE CASCADE,
    network_id UUID NOT NULL REFERENCES public.networks(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX organization_permissions_org_slug_alive_idx
    ON public.organization_permissions (organization_id, slug)
    WHERE deleted_at IS NULL;

CREATE TABLE public.organization_permission_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    permission_id UUID NOT NULL REFERENCES public.organization_permissions(id) ON DELETE CASCADE,
    resource TEXT NOT NULL,
    actions TEXT[] NOT NULL,
    resource_id UUID,
    schema_id UUID REFERENCES public.schemas(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE public.organization_permission_fields (
    grant_id UUID NOT NULL REFERENCES public.organization_permission_grants(id) ON DELETE CASCADE,
    field_name TEXT NOT NULL,
    access TEXT NOT NULL CHECK (access IN ('read', 'write')),
    PRIMARY KEY (grant_id, field_name)
);

CREATE TABLE public.organization_group_permissions (
    group_id UUID NOT NULL REFERENCES public.organization_groups(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES public.organization_permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, permission_id)
);

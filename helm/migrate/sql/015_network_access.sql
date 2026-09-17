CREATE TABLE public.network_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    network_id UUID NOT NULL REFERENCES public.networks(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX network_groups_network_slug_alive_idx
    ON public.network_groups (network_id, slug)
    WHERE deleted_at IS NULL;

CREATE TABLE public.network_group_members (
    group_id UUID NOT NULL REFERENCES public.network_groups(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE public.network_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    network_id UUID NOT NULL REFERENCES public.networks(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX network_permissions_network_slug_alive_idx
    ON public.network_permissions (network_id, slug)
    WHERE deleted_at IS NULL;

CREATE TABLE public.network_permission_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    permission_id UUID NOT NULL REFERENCES public.network_permissions(id) ON DELETE CASCADE,
    resource TEXT NOT NULL,
    actions TEXT[] NOT NULL,
    resource_id UUID,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE public.network_group_permissions (
    group_id UUID NOT NULL REFERENCES public.network_groups(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES public.network_permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, permission_id)
);

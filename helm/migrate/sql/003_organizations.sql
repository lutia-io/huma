CREATE TABLE public.organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    network_id UUID NOT NULL REFERENCES public.networks(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES public.users(id),
    created_by UUID NOT NULL REFERENCES public.users(id),
    updated_by UUID NOT NULL REFERENCES public.users(id),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (network_id, slug),
    UNIQUE (id, network_id)
);

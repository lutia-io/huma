CREATE INDEX records_network_schema_idx
    ON public.records (network_id, schema_id)
    WHERE deleted_at IS NULL;

CREATE INDEX records_network_organization_schema_idx
    ON public.records (network_id, organization_id, schema_id)
    WHERE deleted_at IS NULL;

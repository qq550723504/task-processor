CREATE TABLE IF NOT EXISTS public.organization_source_accounts (
    id BIGINT NOT NULL,
    organization_id VARCHAR(128) NOT NULL
        CONSTRAINT organization_source_accounts_organization_nonempty
        CHECK (organization_id <> '' AND organization_id = btrim(organization_id)),
    platform VARCHAR(32) NOT NULL
        CONSTRAINT organization_source_accounts_platform_1688 CHECK (platform = '1688'),
    label VARCHAR(128),
    profile_ref VARCHAR(256) NOT NULL
        CONSTRAINT organization_source_accounts_profile_ref_nonempty CHECK (btrim(profile_ref) <> ''),
    profile_directory VARCHAR(1024) NOT NULL
        CONSTRAINT organization_source_accounts_profile_directory_nonempty CHECK (btrim(profile_directory) <> ''),
    proxy_ref VARCHAR(256),
    login_url TEXT,
    status SMALLINT NOT NULL,
    deleted SMALLINT NOT NULL,
    last_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT organization_source_accounts_pkey PRIMARY KEY (id),
    CONSTRAINT organization_source_accounts_id_positive CHECK (id > 0)
);

CREATE INDEX IF NOT EXISTS idx_organization_source_accounts_reader
    ON public.organization_source_accounts (organization_id, id, status, deleted);

CREATE TABLE IF NOT EXISTS public.source_account_ownership_migration_receipts (
    contract_version SMALLINT NOT NULL
        CONSTRAINT source_account_ownership_receipts_version_one CHECK (contract_version = 1),
    idempotency_key VARCHAR(128) NOT NULL
        CONSTRAINT source_account_ownership_receipts_key_bytes CHECK (octet_length(idempotency_key) BETWEEN 1 AND 128),
    stage VARCHAR(32) NOT NULL
        CONSTRAINT source_account_ownership_receipts_stage_prepared CHECK (stage = 'prepared_only'),
    request_sha256 CHAR(64) NOT NULL
        CONSTRAINT source_account_ownership_receipts_request_hash CHECK (request_sha256 ~ '^[0-9a-f]{64}$'),
    preflight_sha256 CHAR(64) NOT NULL
        CONSTRAINT source_account_ownership_receipts_preflight_hash CHECK (preflight_sha256 ~ '^[0-9a-f]{64}$'),
    source_id VARCHAR(256) NOT NULL
        CONSTRAINT source_account_ownership_receipts_source_id_nonempty CHECK (source_id <> ''),
    source_database VARCHAR(128) NOT NULL
        CONSTRAINT source_account_ownership_receipts_database_nonempty CHECK (source_database <> ''),
    source_schema VARCHAR(63) NOT NULL
        CONSTRAINT source_account_ownership_receipts_public_schema CHECK (source_schema = 'public'),
    source_sha256 CHAR(64) NOT NULL
        CONSTRAINT source_account_ownership_receipts_source_hash CHECK (source_sha256 ~ '^[0-9a-f]{64}$'),
    target_sha256 CHAR(64) NOT NULL
        CONSTRAINT source_account_ownership_receipts_target_hash CHECK (target_sha256 ~ '^[0-9a-f]{64}$'),
    account_count INTEGER NOT NULL
        CONSTRAINT source_account_ownership_receipts_account_count CHECK (account_count BETWEEN 1 AND 100000),
    result_json JSONB NOT NULL,
    prepared_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT source_account_ownership_migration_receipts_pkey
        PRIMARY KEY (contract_version, idempotency_key)
);

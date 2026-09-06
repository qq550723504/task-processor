-- Explicit operator-applied schema; no runtime migrations. #319 applies this
-- only to its isolated PostgreSQL. There are no legacy imports or backfills.
CREATE TABLE listing_shein_records (
 id uuid PRIMARY KEY,
 organization_id varchar(64) NOT NULL CHECK (length(btrim(organization_id)) > 0),
 owner_user_id varchar(128) NOT NULL CHECK (length(btrim(owner_user_id)) > 0),
 operation_id varchar(128) NOT NULL CHECK (length(btrim(operation_id)) > 0),
 product_key varchar(128) NOT NULL CHECK (length(btrim(product_key)) > 0),
 snapshot_version bigint NOT NULL CHECK (snapshot_version > 0),
 country varchar(2) NOT NULL CHECK (country = 'US'),
 language varchar(2) NOT NULL CHECK (language = 'en'),
 payload bytea NOT NULL CHECK (octet_length(payload) BETWEEN 1 AND 2097152),
 created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE (organization_id, owner_user_id, operation_id)
);

-- The collection has two authorized read shapes. Keep both keyset paths
-- bounded without loading payload: operator (organization + owner) and
-- tenant admin (organization only).
CREATE INDEX listing_shein_records_owner_collection_idx
 ON listing_shein_records (organization_id, owner_user_id, created_at DESC, id DESC);
CREATE INDEX listing_shein_records_admin_collection_idx
 ON listing_shein_records (organization_id, created_at DESC, id DESC);

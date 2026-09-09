-- Explicit operator-applied greenfield schema for DRAFT-S1. There is no
-- runtime migration, compatibility table, fallback read, backfill or dual write.
CREATE TABLE listing_shein_records (
 id uuid PRIMARY KEY,
 organization_id varchar(200) NOT NULL CHECK (length(btrim(organization_id)) > 0),
 owner_user_id varchar(200) NOT NULL CHECK (length(btrim(owner_user_id)) > 0),
 operation_id varchar(128) NOT NULL CHECK (length(btrim(operation_id)) > 0),
 product_key varchar(128) NOT NULL CHECK (length(btrim(product_key)) > 0),
 snapshot_version bigint NOT NULL CHECK (snapshot_version > 0),
 store_id uuid NOT NULL,
 country varchar(2) NOT NULL CHECK (country = 'US'),
 language varchar(2) NOT NULL CHECK (language = 'en'),
 action varchar(16) NOT NULL CHECK (action IN ('save_draft', 'publish')),
 input_hash varchar(71) NOT NULL CHECK (input_hash LIKE 'sha256:%'),
 product_hash varchar(71) NOT NULL CHECK (product_hash LIKE 'sha256:%'),
 asset_inventory_version bigint NOT NULL CHECK (asset_inventory_version = snapshot_version),
 asset_inventory_hash varchar(71) NOT NULL CHECK (asset_inventory_hash LIKE 'sha256:%'),
 rule_revision varchar(128) NOT NULL CHECK (length(btrim(rule_revision)) > 0),
 policy_revision varchar(128) NOT NULL CHECK (length(btrim(policy_revision)) > 0),
 package_hash varchar(71) NOT NULL CHECK (package_hash LIKE 'sha256:%'),
 diagnostic_hash varchar(71) NOT NULL CHECK (diagnostic_hash LIKE 'sha256:%'),
 diagnostic_status varchar(32) NOT NULL CHECK (length(btrim(diagnostic_status)) > 0),
 payload bytea NOT NULL CHECK (octet_length(payload) BETWEEN 1 AND 2097152),
 diagnostic bytea NOT NULL CHECK (octet_length(diagnostic) BETWEEN 1 AND 2097152),
 created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- This is the sole operation receipt and idempotency owner. It is inserted
-- before its record inside one transaction; the deferred foreign key makes the
-- pair atomic while the organization-scoped key serializes concurrent replays.
CREATE TABLE listing_shein_record_operations (
 organization_id varchar(200) NOT NULL CHECK (length(btrim(organization_id)) > 0),
 operation_id varchar(128) NOT NULL CHECK (length(btrim(operation_id)) > 0),
 owner_user_id varchar(200) NOT NULL CHECK (length(btrim(owner_user_id)) > 0),
 input_hash varchar(71) NOT NULL CHECK (input_hash LIKE 'sha256:%'),
 record_id uuid NOT NULL UNIQUE,
 created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (organization_id, operation_id),
 CONSTRAINT listing_shein_record_operations_record_fk
  FOREIGN KEY (record_id) REFERENCES listing_shein_records(id)
  DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX listing_shein_records_owner_collection_idx
 ON listing_shein_records (organization_id, owner_user_id, created_at DESC, id DESC);
CREATE INDEX listing_shein_records_admin_collection_idx
 ON listing_shein_records (organization_id, created_at DESC, id DESC);

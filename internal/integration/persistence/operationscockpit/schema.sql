CREATE SCHEMA operations_cockpit;
REVOKE ALL ON SCHEMA operations_cockpit FROM PUBLIC;

CREATE TABLE operations_cockpit.facts (
    organization_id varchar(200) NOT NULL,
    record_id uuid NOT NULL,
    store_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    start_date date NOT NULL,
    end_date date NOT NULL CHECK (end_date >= start_date AND end_date - start_date < 366),
    amounts jsonb NOT NULL CHECK (jsonb_typeof(amounts) = 'object' AND octet_length(amounts::text) <= 8192),
    note text NOT NULL CHECK (length(note) <= 1000),
    updated_by varchar(200) NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (organization_id, record_id)
);
CREATE INDEX facts_scope_period ON operations_cockpit.facts(organization_id, store_id, start_date, end_date);
CREATE TABLE operations_cockpit.fact_versions (LIKE operations_cockpit.facts INCLUDING DEFAULTS INCLUDING CONSTRAINTS);
ALTER TABLE operations_cockpit.fact_versions ADD PRIMARY KEY (organization_id, record_id, revision);

CREATE TABLE operations_cockpit.goal_heads (
    organization_id varchar(200) PRIMARY KEY,
    goal_id uuid NOT NULL,
    creator_id varchar(200) NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    config jsonb NOT NULL CHECK (jsonb_typeof(config) = 'object' AND octet_length(config::text) <= 16384),
    updated_by varchar(200) NOT NULL,
    updated_at timestamptz NOT NULL,
    UNIQUE (organization_id, goal_id)
);
CREATE TABLE operations_cockpit.goal_versions (
    organization_id varchar(200) NOT NULL,
    goal_id uuid NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    config jsonb NOT NULL CHECK (jsonb_typeof(config) = 'object' AND octet_length(config::text) <= 16384),
    updated_by varchar(200) NOT NULL,
    updated_at timestamptz NOT NULL,
    PRIMARY KEY (organization_id, goal_id, revision),
    FOREIGN KEY (organization_id, goal_id) REFERENCES operations_cockpit.goal_heads(organization_id, goal_id)
);
CREATE TABLE operations_cockpit.commands (
    organization_id varchar(200) NOT NULL,
    actor_id varchar(200) NOT NULL,
    idempotency_key uuid NOT NULL,
    fingerprint char(64) NOT NULL,
    store_ids jsonb NOT NULL CHECK (jsonb_typeof(store_ids) = 'array' AND jsonb_array_length(store_ids) BETWEEN 1 AND 50),
    receipt jsonb NOT NULL CHECK (jsonb_typeof(receipt) = 'object' AND octet_length(receipt::text) <= 8192),
    PRIMARY KEY (organization_id, actor_id, idempotency_key)
);

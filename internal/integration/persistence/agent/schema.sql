CREATE TABLE IF NOT EXISTS product_agent_runs (
    run_id varchar(128) PRIMARY KEY,
    org varchar(128) NOT NULL,
    actor varchar(128) NOT NULL,
    context_kind varchar(128) NOT NULL,
    context_id varchar(128) NOT NULL,
    request_key varchar(128) NOT NULL,
    fingerprint varchar(64) NOT NULL,
    revision bigint NOT NULL CHECK (revision > 0),
    phase varchar(32) NOT NULL CHECK (phase IN ('running', 'interrupted', 'human_review_required', 'stopped')),
    payload bytea NOT NULL CHECK (octet_length(payload) <= 2097152),
    UNIQUE (org, actor, context_kind, context_id, request_key)
);

CREATE TABLE IF NOT EXISTS product_agent_tool_calls (
    run_id varchar(128) NOT NULL REFERENCES product_agent_runs(run_id),
    call_id varchar(128) NOT NULL,
    payload bytea NOT NULL CHECK (octet_length(payload) <= 8192),
    PRIMARY KEY (run_id, call_id)
);

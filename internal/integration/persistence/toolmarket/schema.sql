CREATE TABLE tool_market.activations (
 organization_id varchar(128) NOT NULL, tool_id varchar(80) NOT NULL,
 enabled boolean NOT NULL, revision bigint NOT NULL CHECK(revision>0),
 updated_by varchar(128) NOT NULL, updated_at timestamptz NOT NULL,
 PRIMARY KEY(organization_id,tool_id)
);
CREATE TABLE tool_market.requests (
 id uuid PRIMARY KEY, organization_id varchar(128) NOT NULL, created_by varchar(128) NOT NULL,
 kind varchar(20) NOT NULL CHECK(kind IN ('DATA','CONNECTION','AUTOMATION','OUTPUT')),
 title varchar(120) NOT NULL, description varchar(4000) NOT NULL,
 stage varchar(20) NOT NULL CHECK(stage IN ('SUBMITTED','EVALUATING','PLAN_CONFIRMED','DEVELOPING','DELIVERED','CLOSED')),
 revision bigint NOT NULL CHECK(revision>0), created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL
);
CREATE INDEX requests_org_page ON tool_market.requests(organization_id,id);
CREATE TABLE tool_market.events (
 request_id uuid NOT NULL REFERENCES tool_market.requests(id), revision bigint NOT NULL CHECK(revision>0),
 stage varchar(20) NOT NULL, note varchar(2000) NOT NULL, actor_id varchar(128) NOT NULL, occurred_at timestamptz NOT NULL,
 PRIMARY KEY(request_id,revision)
);
CREATE TABLE tool_market.commands (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, idempotency_key uuid NOT NULL,
 fingerprint char(64) NOT NULL, receipt jsonb,
 PRIMARY KEY(organization_id,actor_id,idempotency_key)
);
REVOKE ALL ON SCHEMA tool_market FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA tool_market FROM PUBLIC;

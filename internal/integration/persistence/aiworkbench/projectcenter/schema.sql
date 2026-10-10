CREATE SCHEMA IF NOT EXISTS ai_workbench_projects;
CREATE TABLE IF NOT EXISTS ai_workbench_projects.projects (
 id uuid PRIMARY KEY, organization_id text NOT NULL, actor_id text NOT NULL,
 title text NOT NULL CHECK(octet_length(title) BETWEEN 1 AND 256),
 goal text NOT NULL CHECK(octet_length(goal) BETWEEN 1 AND 4096),
 kind text NOT NULL CHECK(kind IN ('STORE_OPERATIONS','PRODUCT_DEVELOPMENT','PRODUCT_RESEARCH','BRAND_BUILDING','OPC','OTHER')),
 store_id text NOT NULL DEFAULT '', due_date text NOT NULL DEFAULT '',
 archived boolean NOT NULL DEFAULT false, revision bigint NOT NULL CHECK(revision>0),
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 CHECK(octet_length(organization_id) BETWEEN 1 AND 128), CHECK(octet_length(actor_id) BETWEEN 1 AND 128),
 CHECK(octet_length(store_id)<=128), CHECK(due_date='' OR due_date ~ '^[0-9]{4}-[0-9]{2}-[0-9]{2}$'),
 UNIQUE(id,organization_id,actor_id)
);
CREATE INDEX IF NOT EXISTS project_owner_recent ON ai_workbench_projects.projects(organization_id,actor_id,archived,updated_at DESC,id DESC);
CREATE TABLE IF NOT EXISTS ai_workbench_projects.references (
 slot_id uuid PRIMARY KEY, project_id uuid NOT NULL, organization_id text NOT NULL, actor_id text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('CONVERSATION','BUSINESS_TASK','PRODUCT','KNOWLEDGE_BASE','KNOWLEDGE_SOURCE')),
 target_id uuid NOT NULL, active boolean NOT NULL DEFAULT true,
 FOREIGN KEY(project_id,organization_id,actor_id) REFERENCES ai_workbench_projects.projects(id,organization_id,actor_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS project_active_reference ON ai_workbench_projects.references(project_id,kind,target_id) WHERE active;
CREATE TABLE IF NOT EXISTS ai_workbench_projects.visits (
 project_id uuid PRIMARY KEY, organization_id text NOT NULL, actor_id text NOT NULL,last_visited_at timestamptz NOT NULL,
 FOREIGN KEY(project_id,organization_id,actor_id) REFERENCES ai_workbench_projects.projects(id,organization_id,actor_id)
);
CREATE TABLE IF NOT EXISTS ai_workbench_projects.templates (
 id uuid PRIMARY KEY, organization_id text NOT NULL, actor_id text NOT NULL,
 name text NOT NULL CHECK(octet_length(name) BETWEEN 1 AND 256),
 title text NOT NULL CHECK(octet_length(title) BETWEEN 1 AND 256),goal text NOT NULL CHECK(octet_length(goal) BETWEEN 1 AND 4096),
 kind text NOT NULL CHECK(kind IN ('STORE_OPERATIONS','PRODUCT_DEVELOPMENT','PRODUCT_RESEARCH','BRAND_BUILDING','OPC','OTHER')),
 archived boolean NOT NULL DEFAULT false,revision bigint NOT NULL CHECK(revision>0),created_at timestamptz NOT NULL,
 CHECK(octet_length(organization_id) BETWEEN 1 AND 128), CHECK(octet_length(actor_id) BETWEEN 1 AND 128)
);
CREATE INDEX IF NOT EXISTS template_owner_recent ON ai_workbench_projects.templates(organization_id,actor_id,archived,created_at DESC,id DESC);
CREATE TABLE IF NOT EXISTS ai_workbench_projects.commands (
 organization_id text NOT NULL,actor_id text NOT NULL,key uuid NOT NULL,operation text NOT NULL,
 fingerprint char(64) NOT NULL,entity_id uuid NOT NULL,revision bigint NOT NULL CHECK(revision>0),created_at timestamptz NOT NULL,
 PRIMARY KEY(organization_id,actor_id,key)
);
CREATE TABLE IF NOT EXISTS ai_workbench_projects.audit (
 id uuid PRIMARY KEY,organization_id text NOT NULL,actor_id text NOT NULL,entity_id uuid NOT NULL,
 operation text NOT NULL,revision bigint NOT NULL CHECK(revision>0),created_at timestamptz NOT NULL
);
REVOKE ALL ON SCHEMA ai_workbench_projects FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA ai_workbench_projects FROM PUBLIC;

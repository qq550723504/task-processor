CREATE TABLE public.knowledge_bases (
 organization_id varchar(128) NOT NULL CHECK (length(organization_id)>0), id uuid NOT NULL,
 name varchar(120) NOT NULL CHECK (length(name)>0 AND name=btrim(name) AND name !~ '[[:cntrl:]]'),
 state varchar(16) NOT NULL CHECK (state IN ('ACTIVE','DISABLING','DISABLED')),
 version bigint NOT NULL CHECK (version>0), fence_version bigint NOT NULL CHECK (fence_version>0),
 created_by varchar(256) NOT NULL, updated_by varchar(256) NOT NULL,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 PRIMARY KEY (organization_id,id)
);
CREATE TABLE public.knowledge_sources (
 organization_id varchar(128) NOT NULL, id uuid NOT NULL, base_id uuid NOT NULL,
 name varchar(120) NOT NULL CHECK (length(name)>0 AND name=btrim(name) AND name !~ '[[:cntrl:]]'),
 state varchar(16) NOT NULL CHECK (state IN ('ACTIVE','DISABLING','DISABLED')),
 version bigint NOT NULL CHECK (version>0), fence_version bigint NOT NULL CHECK (fence_version>0),
 latest_revision_id uuid, current_readable_revision_id uuid,
 created_by varchar(256) NOT NULL, updated_by varchar(256) NOT NULL,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 PRIMARY KEY (organization_id,id),
 FOREIGN KEY (organization_id,base_id) REFERENCES public.knowledge_bases(organization_id,id)
);
CREATE INDEX knowledge_sources_base ON public.knowledge_sources(organization_id,base_id,state);
CREATE TABLE public.knowledge_revisions (
 organization_id varchar(128) NOT NULL, id uuid NOT NULL, source_id uuid NOT NULL,
 number bigint NOT NULL CHECK (number>0), filename varchar(255) NOT NULL CHECK (octet_length(filename)<=255),
 content_type varchar(128) NOT NULL CHECK (content_type IN ('text/plain','text/markdown','application/pdf','application/vnd.openxmlformats-officedocument.wordprocessingml.document')),
 size_bytes bigint NOT NULL CHECK (size_bytes BETWEEN 1 AND 10485760),
 sha256 char(64) NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'), object_key varchar(512) NOT NULL,
 state varchar(16) NOT NULL CHECK (state IN ('ADMITTED','OBJECT_STORED','PROCESSING','AVAILABLE','PARTIAL','FAILED')),
 failure varchar(64) NOT NULL DEFAULT '', warning varchar(64) NOT NULL DEFAULT '',
 text text NOT NULL DEFAULT '' CHECK (octet_length(text)<=2097152),
 lease_owner varchar(128) NOT NULL DEFAULT '', lease_until timestamptz,
 attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 3), next_attempt_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 PRIMARY KEY (organization_id,id), UNIQUE(organization_id,source_id,id), UNIQUE(organization_id,source_id,number),
 FOREIGN KEY (organization_id,source_id) REFERENCES public.knowledge_sources(organization_id,id),
 CHECK ((state NOT IN ('AVAILABLE','PARTIAL')) OR (length(text)>0 AND failure='')),
 CHECK (state<>'PARTIAL' OR warning<>'')
);
CREATE UNIQUE INDEX knowledge_one_nonterminal_revision ON public.knowledge_revisions(organization_id,source_id) WHERE state IN ('ADMITTED','OBJECT_STORED','PROCESSING');
CREATE INDEX knowledge_revisions_recovery ON public.knowledge_revisions(next_attempt_at,lease_until) WHERE state IN ('ADMITTED','OBJECT_STORED','PROCESSING');
ALTER TABLE public.knowledge_sources ADD CONSTRAINT knowledge_latest_revision FOREIGN KEY(organization_id,id,latest_revision_id) REFERENCES public.knowledge_revisions(organization_id,source_id,id) DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE public.knowledge_sources ADD CONSTRAINT knowledge_current_revision FOREIGN KEY(organization_id,id,current_readable_revision_id) REFERENCES public.knowledge_revisions(organization_id,source_id,id) DEFERRABLE INITIALLY DEFERRED;
CREATE TABLE public.knowledge_ingest_operations (
 organization_id varchar(128) NOT NULL, kind varchar(32) NOT NULL CHECK(kind IN ('base_create','base_update','base_disable','source_create','revision_create','source_disable')),
 key uuid NOT NULL, actor_id varchar(256) NOT NULL,
 fingerprint char(64) NOT NULL CHECK(fingerprint ~ '^[0-9a-f]{64}$'),
 result jsonb NOT NULL CHECK(octet_length(result::text)<=16384),
 source_id uuid, revision_id uuid, created_at timestamptz NOT NULL,
 PRIMARY KEY(organization_id,kind,key),
 FOREIGN KEY(organization_id,source_id) REFERENCES public.knowledge_sources(organization_id,id),
 FOREIGN KEY(organization_id,revision_id) REFERENCES public.knowledge_revisions(organization_id,id)
);
-- Greenfield install only. No serving DDL or retained-project migration.
ALTER TABLE public.knowledge_sources ADD CONSTRAINT knowledge_source_base_identity UNIQUE(organization_id,base_id,id);
CREATE TABLE public.knowledge_context_bundles (
 organization_id varchar(128) NOT NULL, id uuid NOT NULL, actor_id varchar(256) NOT NULL,
 context_kind varchar(128) NOT NULL, context_id varchar(128) NOT NULL, request_key varchar(128) NOT NULL,
 fingerprint char(64) NOT NULL CHECK(fingerprint ~ '^[0-9a-f]{64}$'),
 selection varchar(51) NOT NULL, policy_version varchar(128) NOT NULL,
 base_id uuid NOT NULL, base_fence_version bigint NOT NULL CHECK(base_fence_version>0),
 payload bytea NOT NULL CHECK(octet_length(payload) BETWEEN 1 AND 98304),
 digest char(64) NOT NULL CHECK(digest ~ '^[0-9a-f]{64}$'), created_at timestamptz NOT NULL,
 PRIMARY KEY(organization_id,id), UNIQUE(organization_id,base_id,id), UNIQUE(organization_id,id,digest),
 UNIQUE(organization_id,actor_id,context_kind,context_id,request_key),
 FOREIGN KEY(organization_id,base_id) REFERENCES public.knowledge_bases(organization_id,id),
 CHECK(length(actor_id)>0 AND length(context_kind)>0 AND length(context_id)>0 AND length(request_key)>0 AND length(policy_version)>0),
 CHECK(selection='knowledge-base:' || base_id::text)
);
CREATE TABLE public.knowledge_context_bundle_entries (
 organization_id varchar(128) NOT NULL, bundle_id uuid NOT NULL, base_id uuid NOT NULL,
 source_id uuid NOT NULL, revision_id uuid NOT NULL, citation_id uuid NOT NULL,
 source_fence_version bigint NOT NULL CHECK(source_fence_version>0),
 content_digest char(64) NOT NULL CHECK(content_digest ~ '^[0-9a-f]{64}$'),
 PRIMARY KEY(organization_id,bundle_id,source_id), UNIQUE(organization_id,bundle_id,citation_id),
 FOREIGN KEY(organization_id,base_id,bundle_id) REFERENCES public.knowledge_context_bundles(organization_id,base_id,id),
 FOREIGN KEY(organization_id,base_id,source_id) REFERENCES public.knowledge_sources(organization_id,base_id,id),
 FOREIGN KEY(organization_id,source_id,revision_id) REFERENCES public.knowledge_revisions(organization_id,source_id,id)
);
CREATE TABLE public.knowledge_dispatch_permits (
 organization_id varchar(128) NOT NULL, id uuid NOT NULL, actor_id varchar(256) NOT NULL,
 invocation_id varchar(128) NOT NULL CHECK(length(invocation_id)>0), bundle_id uuid NOT NULL,
 digest char(64) NOT NULL, state varchar(16) NOT NULL CHECK(state IN ('ACTIVE','RELEASED','EXPIRED')),
 acquired_at timestamptz NOT NULL, expires_at timestamptz NOT NULL,
 PRIMARY KEY(organization_id,id), UNIQUE(organization_id,invocation_id),
 FOREIGN KEY(organization_id,bundle_id,digest) REFERENCES public.knowledge_context_bundles(organization_id,id,digest),
 CHECK(expires_at>acquired_at AND expires_at<=acquired_at+interval '5 minutes 30 seconds')
);
CREATE INDEX knowledge_permits_expiry ON public.knowledge_dispatch_permits(expires_at) WHERE state='ACTIVE';
CREATE INDEX knowledge_permits_bundle ON public.knowledge_dispatch_permits(organization_id,bundle_id,state,expires_at);

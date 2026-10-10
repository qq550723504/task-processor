CREATE SCHEMA shein_observations;
CREATE TABLE shein_observations.commands(
 organization_id varchar(200) NOT NULL,
 id uuid NOT NULL,
 actor_id varchar(200) NOT NULL,
 member_id varchar(200) NOT NULL,
 command_key uuid NOT NULL,
 payload_hash char(64) NOT NULL,
 input jsonb NOT NULL CHECK(jsonb_typeof(input)='object' AND octet_length(input::text)<=131072),
 created_at timestamptz NOT NULL,
 PRIMARY KEY(organization_id,id),
 UNIQUE(organization_id,actor_id,command_key)
);
CREATE TABLE shein_observations.syncs(
 organization_id varchar(200) NOT NULL,
 store_id uuid NOT NULL,
 id uuid NOT NULL,
 command_id uuid NOT NULL,
 actor_id varchar(200) NOT NULL,
 member_id varchar(200) NOT NULL,
 command_key char(64) NOT NULL,
 kind varchar(16) NOT NULL CHECK(kind IN('products','orders')),
 generation bigint NOT NULL CHECK(generation>0),
 revision bigint NOT NULL CHECK(revision>0),
 status varchar(16) NOT NULL CHECK(status IN('pending','running','completed','partial','failed','suspended')),
 binding jsonb NOT NULL CHECK(jsonb_typeof(binding)='object' AND octet_length(binding::text)<=4096),
 progress jsonb NOT NULL CHECK(jsonb_typeof(progress)='object' AND octet_length(progress::text)<=65536),
 query_range jsonb,
 created_at timestamptz NOT NULL,
 observed_at timestamptz,
 error_code varchar(64) NOT NULL DEFAULT '',
 PRIMARY KEY(organization_id,store_id,id),
 UNIQUE(organization_id,id),
 UNIQUE(organization_id,store_id,kind,id),
 UNIQUE(organization_id,actor_id,command_key),
 UNIQUE(organization_id,store_id,kind,generation),
 FOREIGN KEY(organization_id,command_id) REFERENCES shein_observations.commands(organization_id,id)
);
CREATE TABLE shein_observations.heads(
 organization_id varchar(200) NOT NULL,
 store_id uuid NOT NULL,
 kind varchar(16) NOT NULL CHECK(kind IN('products','orders')),
 sequence bigint NOT NULL CHECK(sequence>0),
 current_id uuid,
 PRIMARY KEY(organization_id,store_id,kind),
 FOREIGN KEY(organization_id,store_id,kind,current_id) REFERENCES shein_observations.syncs(organization_id,store_id,kind,id)
);
CREATE TABLE shein_observations.records(
 organization_id varchar(200) NOT NULL,
 store_id uuid NOT NULL,
 sync_id uuid NOT NULL,
 kind varchar(16) NOT NULL CHECK(kind IN('products','orders')),
 platform_id varchar(128) NOT NULL,
 payload jsonb NOT NULL CHECK(jsonb_typeof(payload)='object' AND octet_length(payload::text)<=262144),
 observed_at timestamptz NOT NULL,
 window_key varchar(100) NOT NULL DEFAULT '',
 page_index integer NOT NULL CHECK(page_index>0),
 PRIMARY KEY(organization_id,store_id,sync_id,platform_id),
 FOREIGN KEY(organization_id,store_id,kind,sync_id) REFERENCES shein_observations.syncs(organization_id,store_id,kind,id),
 CHECK((kind='products' AND payload->'product'->>'id'=platform_id AND NOT payload ? 'order') OR (kind='orders' AND payload->'order'->>'id'=platform_id AND payload->'order'->>'site'='shein-us' AND NOT payload ? 'product'))
);
CREATE INDEX observations_sync_generation ON shein_observations.syncs(organization_id,store_id,kind,generation DESC);

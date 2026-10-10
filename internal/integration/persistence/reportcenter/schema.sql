CREATE TABLE report_center.saved_reports (
 id uuid PRIMARY KEY, organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL,
 kind varchar(20) NOT NULL CHECK(kind IN ('TITLE_REVIEW','SHEIN_RECORD')),
 source_id uuid NOT NULL, source_version varchar(96) NOT NULL,
 title varchar(256) NOT NULL, product_key varchar(128) NOT NULL, store_id varchar(36) NOT NULL,
 source_at timestamptz, captured_at timestamptz NOT NULL,
 content bytea NOT NULL CHECK(octet_length(content)<=131072), digest char(64) NOT NULL,
 UNIQUE(organization_id,actor_id,kind,source_id,source_version),
 UNIQUE(id,organization_id,actor_id)
);
CREATE TABLE report_center.favorites (
 report_id uuid PRIMARY KEY, organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL,
 favorite boolean NOT NULL, updated_at timestamptz NOT NULL,
 FOREIGN KEY(report_id,organization_id,actor_id) REFERENCES report_center.saved_reports(id,organization_id,actor_id)
);
CREATE TABLE report_center.commands (
 organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL, idempotency_key uuid NOT NULL,
 fingerprint char(64) NOT NULL, report_id uuid NOT NULL,
 PRIMARY KEY(organization_id,actor_id,idempotency_key),
 FOREIGN KEY(report_id,organization_id,actor_id) REFERENCES report_center.saved_reports(id,organization_id,actor_id)
);
CREATE INDEX reports_personal_page ON report_center.saved_reports(organization_id,actor_id,captured_at DESC,id DESC);
REVOKE ALL ON SCHEMA report_center FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA report_center FROM PUBLIC;

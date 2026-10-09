CREATE SCHEMA IF NOT EXISTS notification_center;
CREATE TABLE IF NOT EXISTS notification_center.announcements (
 id uuid PRIMARY KEY, realm text NOT NULL, publisher text NOT NULL,
 revision bigint NOT NULL CHECK(revision IN (1,2)),
 state text NOT NULL CHECK(state IN ('PUBLISHED','WITHDRAWN')),
 content jsonb NOT NULL CHECK(octet_length(content::text)<=65536),
 published_at timestamptz NOT NULL, withdrawn_at timestamptz,
 CHECK((state='PUBLISHED' AND revision=1 AND withdrawn_at IS NULL) OR
       (state='WITHDRAWN' AND revision=2 AND withdrawn_at IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS notification_announcements_audience ON notification_center.announcements(realm,state,id);
CREATE TABLE IF NOT EXISTS notification_center.reading_receipts (
 realm text NOT NULL, subject text NOT NULL, reference_key text NOT NULL,
 reference jsonb NOT NULL, read_at timestamptz NOT NULL,
 PRIMARY KEY(realm,subject,reference_key)
);
CREATE TABLE IF NOT EXISTS notification_center.commands (
 scope_key text NOT NULL, command_key uuid NOT NULL, operation text NOT NULL,
 fingerprint text NOT NULL CHECK(length(fingerprint)=64),
 result_id text NOT NULL, committed_at timestamptz NOT NULL,
 PRIMARY KEY(scope_key,command_key)
);
CREATE TABLE IF NOT EXISTS notification_center.snapshots (
 id uuid PRIMARY KEY, scope_key text NOT NULL, fingerprint text NOT NULL CHECK(length(fingerprint)=64),
 refs jsonb NOT NULL CHECK(jsonb_typeof(refs)='array' AND jsonb_array_length(refs)<=10000 AND octet_length(refs::text)<=8388608),
 created_at timestamptz NOT NULL, expires_at timestamptz NOT NULL,
 CHECK(expires_at>created_at AND expires_at<=created_at+interval '5 minutes')
);
CREATE INDEX IF NOT EXISTS notification_snapshots_scope ON notification_center.snapshots(scope_key,expires_at);

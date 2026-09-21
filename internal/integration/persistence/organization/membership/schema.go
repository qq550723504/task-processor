package membership

import (
	"context"
	"database/sql"
	"errors"
)

// InstallSchemaTx belongs to explicit initialization, never request handling.
func InstallSchemaTx(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return errors.New("membership schema transaction unavailable")
	}
	_, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS public.organization_member_operations (
 project_id varchar(128) NOT NULL,
 organization_id varchar(128) NOT NULL,
 actor_id varchar(128) NOT NULL,
 operation_key uuid NOT NULL,
 target_user_id varchar(128) NOT NULL,
 invite_email varchar(320),
 fingerprint char(64) NOT NULL,
 revision bigint NOT NULL CHECK (revision > 0),
 active boolean NOT NULL,
 payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object' AND octet_length(payload::text) <= 16384),
 PRIMARY KEY(project_id,organization_id,actor_id,operation_key)
 );
 CREATE UNIQUE INDEX IF NOT EXISTS organization_member_active_target ON public.organization_member_operations(project_id,organization_id,target_user_id) WHERE active;
 CREATE UNIQUE INDEX IF NOT EXISTS organization_member_active_invite_email ON public.organization_member_operations(project_id,organization_id,invite_email) WHERE active AND invite_email IS NOT NULL;
 CREATE TABLE IF NOT EXISTS public.organization_member_audit_events (
  project_id varchar(128) NOT NULL,
  organization_id varchar(128) NOT NULL,
  actor_id varchar(128) NOT NULL,
  target_user_id varchar(128) NOT NULL,
  operation_key uuid NOT NULL,
  operation varchar(32) NOT NULL,
  revision bigint NOT NULL CHECK (revision > 0),
  created_at timestamptz NOT NULL,
  PRIMARY KEY(project_id,organization_id,actor_id,operation_key),
  CONSTRAINT organization_member_audit_operation_check CHECK (operation IN ('invite','role','remove'))
 );
 ALTER TABLE public.organization_member_audit_events ADD COLUMN IF NOT EXISTS project_id varchar(128);
 UPDATE public.organization_member_audit_events AS audit
 SET project_id=(SELECT operation.project_id
                 FROM public.organization_member_operations AS operation
                 WHERE operation.organization_id=audit.organization_id
                   AND operation.actor_id=audit.actor_id
                   AND operation.operation_key=audit.operation_key)
 WHERE audit.project_id IS NULL
   AND (SELECT count(*)
        FROM public.organization_member_operations AS operation
        WHERE operation.organization_id=audit.organization_id
          AND operation.actor_id=audit.actor_id
          AND operation.operation_key=audit.operation_key) = 1;
 DO $$
 BEGIN
  IF EXISTS (SELECT 1 FROM public.organization_member_audit_events WHERE project_id IS NULL) THEN
   RAISE EXCEPTION 'membership audit rows cannot be assigned to a project';
  END IF;
 END $$;
 ALTER TABLE public.organization_member_audit_events ALTER COLUMN project_id SET NOT NULL;
 DO $$
 BEGIN
  IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='public.organization_member_audit_events'::regclass AND conname='organization_member_audit_events_pkey' AND pg_get_constraintdef(oid) <> 'PRIMARY KEY (project_id, organization_id, actor_id, operation_key)') THEN
   ALTER TABLE public.organization_member_audit_events DROP CONSTRAINT organization_member_audit_events_pkey;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='public.organization_member_audit_events'::regclass AND conname='organization_member_audit_events_pkey') THEN
   ALTER TABLE public.organization_member_audit_events ADD CONSTRAINT organization_member_audit_events_pkey PRIMARY KEY (project_id, organization_id, actor_id, operation_key);
  END IF;
 END $$;`)
	return err
}

type schemaReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func verifySchema(ctx context.Context, db schemaReader) error {
	var auditTableCount int
	rows, err := db.QueryContext(ctx, `SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname='organization_member_audit_events' AND c.relkind='r'`)
	if err != nil {
		return err
	}
	if rows.Next() {
		if err := rows.Scan(&auditTableCount); err != nil {
			_ = rows.Close()
			return err
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if auditTableCount != 1 {
		return errors.New("membership audit table missing")
	}
	rows, err = db.QueryContext(ctx, `SELECT pg_catalog.pg_get_constraintdef(oid) FROM pg_catalog.pg_constraint WHERE conrelid='public.organization_member_audit_events'::regclass AND conname='organization_member_audit_events_pkey'`)
	if err != nil {
		return err
	}
	var auditPrimaryKey string
	if rows.Next() {
		if err := rows.Scan(&auditPrimaryKey); err != nil {
			_ = rows.Close()
			return err
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if auditPrimaryKey != "PRIMARY KEY (project_id, organization_id, actor_id, operation_key)" {
		return errors.New("membership audit primary key mismatch")
	}
	auditColumns := map[string]string{
		"project_id": "character varying(128)", "organization_id": "character varying(128)", "actor_id": "character varying(128)",
		"target_user_id": "character varying(128)", "operation_key": "uuid", "operation": "character varying(32)",
		"revision": "bigint", "created_at": "timestamp with time zone",
	}
	rows, err = db.QueryContext(ctx, `SELECT a.attname,pg_catalog.format_type(a.atttypid,a.atttypmod),a.attnotnull FROM pg_catalog.pg_attribute a WHERE a.attrelid='public.organization_member_audit_events'::regclass AND a.attnum>0 AND NOT a.attisdropped`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var name, kind string
		var notNull bool
		if err := rows.Scan(&name, &kind, &notNull); err != nil {
			_ = rows.Close()
			return err
		}
		if auditColumns[name] != kind || !notNull {
			_ = rows.Close()
			return errors.New("membership audit column contract mismatch")
		}
		delete(auditColumns, name)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(auditColumns) != 0 {
		return errors.New("membership audit column missing")
	}
	want := map[string]string{"project_id": "character varying(128)", "organization_id": "character varying(128)", "actor_id": "character varying(128)", "operation_key": "uuid", "target_user_id": "character varying(128)", "invite_email": "character varying(320)", "fingerprint": "character(64)", "revision": "bigint", "active": "boolean", "payload": "jsonb"}
	rows, err = db.QueryContext(ctx, `SELECT a.attname,pg_catalog.format_type(a.atttypid,a.atttypmod),a.attnotnull FROM pg_catalog.pg_attribute a WHERE a.attrelid='public.organization_member_operations'::regclass AND a.attnum>0 AND NOT a.attisdropped`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name, kind string
		var notNull bool
		if err := rows.Scan(&name, &kind, &notNull); err != nil {
			return err
		}
		if want[name] != kind || notNull != (name != "invite_email") {
			return errors.New("membership column contract mismatch")
		}
		delete(want, name)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(want) != 0 {
		return errors.New("membership column missing")
	}
	if err := rows.Close(); err != nil {
		return err
	}
	constraints := map[string]string{
		"organization_member_operations_pkey":           "PRIMARY KEY (project_id, organization_id, actor_id, operation_key)",
		"organization_member_operations_revision_check": "CHECK ((revision > 0))",
		"organization_member_operations_payload_check":  "CHECK (((jsonb_typeof(payload) = 'object'::text) AND (octet_length((payload)::text) <= 16384)))",
	}
	rows, err = db.QueryContext(ctx, `SELECT conname,pg_catalog.pg_get_constraintdef(oid),convalidated,condeferrable FROM pg_catalog.pg_constraint WHERE conrelid='public.organization_member_operations'::regclass`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name, definition string
		var validated, deferrable bool
		if err := rows.Scan(&name, &definition, &validated, &deferrable); err != nil {
			return err
		}
		if constraints[name] != definition || !validated || deferrable {
			return errors.New("membership constraint mismatch")
		}
		delete(constraints, name)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(constraints) != 0 {
		return errors.New("membership constraint missing")
	}
	if err := rows.Close(); err != nil {
		return err
	}
	return verifyReservationIndexes(ctx, db)
}
func verifyReservationIndexes(ctx context.Context, db schemaReader) error {
	want := map[string]string{
		"organization_member_operations_pkey":     "CREATE UNIQUE INDEX organization_member_operations_pkey ON public.organization_member_operations USING btree (project_id, organization_id, actor_id, operation_key)",
		"organization_member_active_target":       "CREATE UNIQUE INDEX organization_member_active_target ON public.organization_member_operations USING btree (project_id, organization_id, target_user_id) WHERE active",
		"organization_member_active_invite_email": "CREATE UNIQUE INDEX organization_member_active_invite_email ON public.organization_member_operations USING btree (project_id, organization_id, invite_email) WHERE (active AND (invite_email IS NOT NULL))",
	}
	rows, err := db.QueryContext(ctx, `SELECT c.relname,pg_catalog.pg_get_indexdef(i.indexrelid) FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid WHERE i.indrelid='public.organization_member_operations'::regclass AND i.indisvalid AND i.indisready AND i.indimmediate`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name, definition string
		if err := rows.Scan(&name, &definition); err != nil {
			return err
		}
		if expected, ok := want[name]; ok {
			if definition != expected {
				return errors.New("membership reservation index mismatch")
			}
			delete(want, name)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(want) != 0 {
		return errors.New("membership reservation index missing")
	}
	return nil
}

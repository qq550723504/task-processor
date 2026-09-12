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
	_, err := tx.ExecContext(ctx, `CREATE TABLE public.organization_member_operations (
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
 CREATE UNIQUE INDEX organization_member_active_target ON public.organization_member_operations(project_id,organization_id,target_user_id) WHERE active;
 CREATE UNIQUE INDEX organization_member_active_invite_email ON public.organization_member_operations(project_id,organization_id,invite_email) WHERE active AND invite_email IS NOT NULL;`)
	return err
}

type schemaReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func verifySchema(ctx context.Context, db schemaReader) error {
	want := map[string]string{"project_id": "character varying(128)", "organization_id": "character varying(128)", "actor_id": "character varying(128)", "operation_key": "uuid", "target_user_id": "character varying(128)", "invite_email": "character varying(320)", "fingerprint": "character(64)", "revision": "bigint", "active": "boolean", "payload": "jsonb"}
	rows, err := db.QueryContext(ctx, `SELECT a.attname,pg_catalog.format_type(a.atttypid,a.atttypmod),a.attnotnull FROM pg_catalog.pg_attribute a WHERE a.attrelid='public.organization_member_operations'::regclass AND a.attnum>0 AND NOT a.attisdropped`)
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

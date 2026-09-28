package membership

import (
	"context"
	"database/sql"
	"errors"
)

func installInvitations(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS public.organization_member_invitations(
 project_id varchar(128) NOT NULL, organization_id varchar(128) NOT NULL, invitation_id uuid NOT NULL,
 contact varchar(200) NOT NULL, state varchar(16) NOT NULL CHECK(state IN ('pending','accepting','accepted','declined','cancelled','expired')),
 revision bigint NOT NULL CHECK(revision>0), expires_at timestamptz NOT NULL,
 payload jsonb NOT NULL CHECK(jsonb_typeof(payload)='object' AND octet_length(payload::text)<=16384),
 PRIMARY KEY(project_id,invitation_id));
 CREATE UNIQUE INDEX IF NOT EXISTS organization_invitation_active_contact ON public.organization_member_invitations(project_id,organization_id,contact) WHERE state IN ('pending','accepting');`)
	return err
}
func verifyInvitations(ctx context.Context, db schemaReader) error {
	columns := map[string]string{"project_id": "character varying(128)", "organization_id": "character varying(128)", "invitation_id": "uuid", "contact": "character varying(200)", "state": "character varying(16)", "revision": "bigint", "expires_at": "timestamp with time zone", "payload": "jsonb"}
	rows, err := db.QueryContext(ctx, `SELECT a.attname,pg_catalog.format_type(a.atttypid,a.atttypmod),a.attnotnull FROM pg_catalog.pg_attribute a WHERE a.attrelid='public.organization_member_invitations'::regclass AND a.attnum>0 AND NOT a.attisdropped`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var name, kind string
		var required bool
		if err = rows.Scan(&name, &kind, &required); err != nil {
			rows.Close()
			return err
		}
		if columns[name] != kind || !required {
			rows.Close()
			return errors.New("invitation column mismatch")
		}
		delete(columns, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(columns) > 0 {
		return errors.New("invitation column missing")
	}
	constraints := map[string]string{
		"organization_member_invitations_pkey":           "PRIMARY KEY (project_id, invitation_id)",
		"organization_member_invitations_state_check":    "CHECK (((state)::text = ANY ((ARRAY['pending'::character varying, 'accepting'::character varying, 'accepted'::character varying, 'declined'::character varying, 'cancelled'::character varying, 'expired'::character varying])::text[])))",
		"organization_member_invitations_revision_check": "CHECK ((revision > 0))",
		"organization_member_invitations_payload_check":  "CHECK (((jsonb_typeof(payload) = 'object'::text) AND (octet_length((payload)::text) <= 16384)))",
	}
	rows, err = db.QueryContext(ctx, `SELECT conname,pg_catalog.pg_get_constraintdef(oid),convalidated,condeferrable FROM pg_catalog.pg_constraint WHERE conrelid='public.organization_member_invitations'::regclass`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var name, definition string
		var validated, deferrable bool
		if err = rows.Scan(&name, &definition, &validated, &deferrable); err != nil {
			rows.Close()
			return err
		}
		if constraints[name] != definition || !validated || deferrable {
			rows.Close()
			return errors.New("invitation constraint mismatch")
		}
		delete(constraints, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(constraints) > 0 {
		return errors.New("invitation constraint missing")
	}
	rows, err = db.QueryContext(ctx, `SELECT pg_catalog.pg_get_indexdef(i.indexrelid) FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class c ON c.oid=i.indexrelid WHERE i.indrelid='public.organization_member_invitations'::regclass AND i.indisunique AND i.indisvalid AND i.indisready AND i.indimmediate AND c.relname IN ('organization_member_invitations_pkey','organization_invitation_active_contact')`)
	if err != nil {
		return err
	}
	defer rows.Close()
	want := map[string]bool{"CREATE UNIQUE INDEX organization_member_invitations_pkey ON public.organization_member_invitations USING btree (project_id, invitation_id)": true, "CREATE UNIQUE INDEX organization_invitation_active_contact ON public.organization_member_invitations USING btree (project_id, organization_id, contact) WHERE ((state)::text = ANY ((ARRAY['pending'::character varying, 'accepting'::character varying])::text[]))": true}
	for rows.Next() {
		var def string
		if err = rows.Scan(&def); err != nil {
			return err
		}
		delete(want, def)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if len(want) > 0 {
		return errors.New("invitation reservation index mismatch")
	}
	return nil
}

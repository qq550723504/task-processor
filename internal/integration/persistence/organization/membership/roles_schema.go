package membership

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func installRoles(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS public.organization_role_slots (
 project_id varchar(128) NOT NULL, organization_id varchar(128) NOT NULL,
 role_key varchar(128) NOT NULL, slot smallint NOT NULL CHECK(slot BETWEEN 1 AND 64),
 PRIMARY KEY(project_id,organization_id,role_key), UNIQUE(project_id,organization_id,slot), UNIQUE(project_id,role_key));
 CREATE TABLE IF NOT EXISTS public.organization_roles (
 project_id varchar(128) NOT NULL, organization_id varchar(128) NOT NULL, role_key varchar(128) NOT NULL,
 name varchar(160) NOT NULL CHECK(length(name)>0), name_key varchar(320) NOT NULL CHECK(length(name_key)>0),
 modules jsonb NOT NULL CHECK(jsonb_typeof(modules)='array' AND jsonb_array_length(modules)<=32 AND octet_length(modules::text)<=4096),
 revision bigint NOT NULL CHECK(revision BETWEEN 1 AND 1000000000), created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 PRIMARY KEY(project_id,organization_id,role_key), UNIQUE(project_id,organization_id,name_key),
 FOREIGN KEY(project_id,organization_id,role_key) REFERENCES public.organization_role_slots(project_id,organization_id,role_key));
 CREATE TABLE IF NOT EXISTS public.organization_role_mutations (
 project_id varchar(128) NOT NULL, organization_id varchar(128) NOT NULL, actor_id varchar(128) NOT NULL,
 operation_key uuid NOT NULL, fingerprint char(64) NOT NULL, result jsonb NOT NULL CHECK(jsonb_typeof(result)='object' AND octet_length(result::text)<=4096),created_at timestamptz NOT NULL,
 PRIMARY KEY(project_id,organization_id,actor_id,operation_key));`)
	return err
}

func verifyRoleSchema(ctx context.Context, db schemaReader) error {
	if err := verifyRoleReadSchema(ctx, db); err != nil {
		return err
	}
	return verifyRoleTable(ctx, db, "organization_role_mutations", map[string]string{
		"project_id": "character varying(128)", "organization_id": "character varying(128)", "actor_id": "character varying(128)", "operation_key": "uuid", "fingerprint": "character(64)", "result": "jsonb", "created_at": "timestamp with time zone",
	}, []string{"PRIMARY KEY (project_id, organization_id, actor_id, operation_key)", "CHECK (((jsonb_typeof(result) = 'object'::text) AND (octet_length((result)::text) <= 4096)))"})
}
func verifyRoleReadSchema(ctx context.Context, db schemaReader) error {
	if err := verifyRoleTable(ctx, db, "organization_role_slots", map[string]string{"project_id": "character varying(128)", "organization_id": "character varying(128)", "role_key": "character varying(128)", "slot": "smallint"}, []string{
		"PRIMARY KEY (project_id, organization_id, role_key)", "UNIQUE (project_id, organization_id, slot)", "UNIQUE (project_id, role_key)", "CHECK (((slot >= 1) AND (slot <= 64)))"}); err != nil {
		return err
	}
	return verifyRoleTable(ctx, db, "organization_roles", map[string]string{"project_id": "character varying(128)", "organization_id": "character varying(128)", "role_key": "character varying(128)", "name": "character varying(160)", "name_key": "character varying(320)", "modules": "jsonb", "revision": "bigint", "created_at": "timestamp with time zone", "updated_at": "timestamp with time zone"}, []string{
		"PRIMARY KEY (project_id, organization_id, role_key)", "UNIQUE (project_id, organization_id, name_key)", "FOREIGN KEY (project_id, organization_id, role_key) REFERENCES organization_role_slots(project_id, organization_id, role_key)",
		"CHECK ((length((name)::text) > 0))", "CHECK ((length((name_key)::text) > 0))", "CHECK (((jsonb_typeof(modules) = 'array'::text) AND (jsonb_array_length(modules) <= 32) AND (octet_length((modules)::text) <= 4096)))", "CHECK (((revision >= 1) AND (revision <= 1000000000)))"})
}
func verifyRoleTable(ctx context.Context, db schemaReader, table string, columns map[string]string, definitions []string) error {
	rows, err := db.QueryContext(ctx, `SELECT attname,pg_catalog.format_type(atttypid,atttypmod),attnotnull FROM pg_attribute WHERE attrelid= $1::regclass AND attnum>0 AND NOT attisdropped`, "public."+table)
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
			return errors.New("role schema column mismatch")
		}
		delete(columns, name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(columns) > 0 {
		return errors.New("role schema column missing")
	}
	constraints := map[string]bool{}
	for _, d := range definitions {
		constraints[strings.Join(strings.Fields(d), " ")] = true
	}
	rows, err = db.QueryContext(ctx, `SELECT pg_get_constraintdef(c.oid),c.convalidated,c.condeferrable,COALESCE(i.indisvalid AND i.indisready AND i.indimmediate,true) FROM pg_constraint c LEFT JOIN pg_index i ON i.indexrelid=c.conindid WHERE c.conrelid=$1::regclass`, "public."+table)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		var valid, deferred, indexValid bool
		if err = rows.Scan(&d, &valid, &deferred, &indexValid); err != nil {
			return err
		}
		d = strings.Join(strings.Fields(d), " ")
		if !constraints[d] || !valid || deferred || !indexValid {
			return errors.New("role schema constraint mismatch")
		}
		delete(constraints, d)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if len(constraints) > 0 {
		return errors.New("role schema constraint missing")
	}
	return nil
}

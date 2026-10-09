package agentcustomizationpersistence

import (
	"context"
	"database/sql"
	"regexp"
	d "task-processor/internal/agentcustomization"
)

// InstallSchema is an explicit installer operation; serving paths never call it.
func InstallSchema(ctx context.Context, db *sql.DB) error {
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for _, query := range []string{
		"DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='agent_customization_owner') THEN CREATE ROLE agent_customization_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS; END IF; END $$",
		"CREATE SCHEMA IF NOT EXISTS agent_customization AUTHORIZATION agent_customization_owner",
	} {
		if _, e = tx.ExecContext(ctx, query); e != nil {
			return e
		}
	}
	var unsafe bool
	if e = tx.QueryRowContext(ctx, "SELECT rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolbypassrls FROM pg_roles WHERE rolname='agent_customization_owner'").Scan(&unsafe); e != nil {
		return e
	}
	if unsafe {
		return d.ErrInvalid
	}
	if e = tx.QueryRowContext(ctx, "SELECT nspowner<>(SELECT oid FROM pg_roles WHERE rolname='agent_customization_owner') FROM pg_namespace WHERE nspname='agent_customization'").Scan(&unsafe); e != nil {
		return e
	}
	if unsafe {
		return d.ErrInvalid
	}
	for _, query := range []string{
		"SET LOCAL ROLE agent_customization_owner",
		"CREATE TABLE IF NOT EXISTS agent_customization.requests(id uuid PRIMARY KEY,organization_id text NOT NULL,version bigint NOT NULL CHECK(version>0),payload jsonb NOT NULL)",
		"CREATE INDEX IF NOT EXISTS customization_org_id ON agent_customization.requests(organization_id,id)",
		"CREATE TABLE IF NOT EXISTS agent_customization.attachments(id uuid PRIMARY KEY,request_id uuid NOT NULL REFERENCES agent_customization.requests(id),digest text NOT NULL,data bytea NOT NULL CHECK(octet_length(data)>0 AND octet_length(data)<=2097152))",
		"CREATE TABLE IF NOT EXISTS agent_customization.events(request_id uuid NOT NULL REFERENCES agent_customization.requests(id),version bigint NOT NULL CHECK(version>0),payload jsonb NOT NULL,PRIMARY KEY(request_id,version))",
		"CREATE TABLE IF NOT EXISTS agent_customization.commands(scope_kind text NOT NULL CHECK(scope_kind IN ('enterprise','platform')),organization_id text NOT NULL,actor_id text NOT NULL,key uuid NOT NULL,fingerprint text NOT NULL,receipt jsonb NOT NULL,PRIMARY KEY(scope_kind,organization_id,actor_id,key))",
		"REVOKE ALL ON SCHEMA agent_customization FROM PUBLIC",
		"REVOKE ALL ON ALL TABLES IN SCHEMA agent_customization FROM PUBLIC",
		"RESET ROLE",
	} {
		if _, e = tx.ExecContext(ctx, query); e != nil {
			return e
		}
	}
	return tx.Commit()
}

// GrantRuntime reuses the existing separated installer/serving-role pattern.
func GrantRuntime(ctx context.Context, db *sql.DB, role string) error {
	if db == nil || !regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`).MatchString(role) {
		return d.ErrInvalid
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var unsafe bool
	e = tx.QueryRowContext(ctx, "SELECT rolsuper OR rolcreaterole OR rolbypassrls OR pg_has_role(oid,'agent_customization_owner','MEMBER') FROM pg_roles WHERE rolname=$1", role).Scan(&unsafe)
	if e != nil {
		return e
	}
	if unsafe {
		return d.ErrInvalid
	}
	quoted := `"` + role + `"`
	for _, query := range []string{
		"REVOKE ALL ON SCHEMA agent_customization FROM " + quoted,
		"REVOKE ALL ON ALL TABLES IN SCHEMA agent_customization FROM " + quoted,
		"GRANT USAGE ON SCHEMA agent_customization TO " + quoted,
		"GRANT SELECT,INSERT ON ALL TABLES IN SCHEMA agent_customization TO " + quoted,
		"GRANT UPDATE(version,payload) ON agent_customization.requests TO " + quoted,
	} {
		if _, e = tx.ExecContext(ctx, query); e != nil {
			return e
		}
	}
	if e = tx.QueryRowContext(ctx, "SELECT has_schema_privilege($1,'agent_customization','CREATE')", role).Scan(&unsafe); e != nil {
		return e
	}
	if unsafe {
		return d.ErrInvalid
	}
	if e = tx.QueryRowContext(ctx, "SELECT has_column_privilege($1,'agent_customization.requests','id','UPDATE') OR has_column_privilege($1,'agent_customization.requests','organization_id','UPDATE')", role).Scan(&unsafe); e != nil {
		return e
	}
	if unsafe {
		return d.ErrInvalid
	}
	for _, table := range []string{"requests", "events", "attachments", "commands"} {
		if e = tx.QueryRowContext(ctx, "SELECT has_table_privilege($1,$2,'DELETE,TRUNCATE')", role, "agent_customization."+table).Scan(&unsafe); e != nil {
			return e
		}
		if unsafe {
			return d.ErrInvalid
		}
		if table != "requests" {
			if e = tx.QueryRowContext(ctx, "SELECT has_any_column_privilege($1,$2,'UPDATE')", role, "agent_customization."+table).Scan(&unsafe); e != nil {
				return e
			}
			if unsafe {
				return d.ErrInvalid
			}
		}
	}
	return tx.Commit()
}

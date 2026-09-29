package knowledge

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"strings"

	"gorm.io/gorm"
)

//go:embed schema.sql
var schemaSQL string

func InstallSchemaTx(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, schemaSQL)
	return err
}

// Install is explicit greenfield DDL; the serving constructor only verifies.
func Install(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return errors.New("knowledge schema requires PostgreSQL")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(schemaSQL).Error })
}
func NewRepository(ctx context.Context, db *gorm.DB) (*Repository, error) {
	if db == nil || db.Dialector.Name() != "postgres" {
		return nil, errors.New("knowledge requires existing PostgreSQL schema")
	}
	for _, table := range []string{"knowledge_bases", "knowledge_sources", "knowledge_revisions", "knowledge_ingest_operations", "knowledge_context_bundles", "knowledge_context_bundle_entries", "knowledge_dispatch_permits"} {
		var name string
		if err := db.WithContext(ctx).Raw("SELECT COALESCE(to_regclass(?)::text,'')", "public."+table).Scan(&name).Error; err != nil || name == "" {
			return nil, errors.New("knowledge schema is not installed")
		}
	}
	return &Repository{db: db}, nil
}

var runtimeColumns = map[string]struct{ insert, update string }{
	"knowledge_bases":                  {"organization_id,id,name,state,version,fence_version,created_by,updated_by,created_at,updated_at", "name,state,version,fence_version,updated_by,updated_at"},
	"knowledge_sources":                {"organization_id,id,base_id,name,state,version,fence_version,latest_revision_id,current_readable_revision_id,created_by,updated_by,created_at,updated_at", "name,state,version,fence_version,latest_revision_id,current_readable_revision_id,updated_by,updated_at"},
	"knowledge_revisions":              {"organization_id,id,source_id,number,filename,content_type,size_bytes,sha256,object_key,state,failure,warning,text,lease_owner,lease_until,attempts,next_attempt_at,created_at,updated_at", "state,failure,warning,text,lease_owner,lease_until,attempts,next_attempt_at,updated_at"},
	"knowledge_ingest_operations":      {"organization_id,kind,key,actor_id,fingerprint,result,source_id,revision_id,created_at", ""},
	"knowledge_context_bundles":        {"organization_id,id,actor_id,context_kind,context_id,request_key,fingerprint,selection,policy_version,base_id,base_fence_version,payload,digest,created_at", ""},
	"knowledge_context_bundle_entries": {"organization_id,bundle_id,base_id,source_id,revision_id,citation_id,source_fence_version,content_digest", ""},
	"knowledge_dispatch_permits":       {"organization_id,id,actor_id,invocation_id,bundle_id,digest,state,acquired_at,expires_at", "state"},
}

func GrantRuntime(ctx context.Context, db *gorm.DB) error {
	for _, statement := range []string{`DO $$ BEGIN EXECUTE format('GRANT CONNECT ON DATABASE %I TO knowledge_runtime',current_database()); END $$`, `GRANT USAGE ON SCHEMA public TO knowledge_runtime`} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			return err
		}
	}
	for table, columns := range runtimeColumns {
		for _, statement := range []string{"GRANT SELECT ON public." + table + " TO knowledge_runtime", "GRANT INSERT (" + columns.insert + ") ON public." + table + " TO knowledge_runtime"} {
			if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
				return err
			}
		}
		if columns.update != "" {
			if err := db.WithContext(ctx).Exec("GRANT UPDATE (" + columns.update + ") ON public." + table + " TO knowledge_runtime").Error; err != nil {
				return err
			}
		}
	}
	return nil
}
func VerifyRuntime(ctx context.Context, db *gorm.DB) error {
	var role string
	var unsafe bool
	if err := db.WithContext(ctx).Raw("SELECT current_user").Scan(&role).Error; err != nil || role != "knowledge_runtime" {
		return errors.New("knowledge runtime role required")
	}
	if err := db.WithContext(ctx).Raw(`SELECT rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR has_database_privilege(current_user,current_database(),'CREATE') OR has_database_privilege(current_user,current_database(),'TEMP') OR has_schema_privilege(current_user,'public','CREATE') OR EXISTS(SELECT 1 FROM pg_auth_members WHERE member=(SELECT oid FROM pg_roles WHERE rolname=current_user)) OR EXISTS(SELECT 1 FROM pg_database WHERE datallowconn AND datname<>current_database() AND has_database_privilege(current_user,oid,'CONNECT')) FROM pg_roles WHERE rolname=current_user`).Scan(&unsafe).Error; err != nil || unsafe {
		return errors.New("knowledge runtime privileges exceed owner boundary")
	}
	for table, columns := range runtimeColumns {
		var allowed bool
		if err := db.WithContext(ctx).Raw("SELECT has_table_privilege(current_user,?,'SELECT') AND NOT has_table_privilege(current_user,?,'DELETE,TRUNCATE,TRIGGER,REFERENCES')", "public."+table, "public."+table).Scan(&allowed).Error; err != nil || !allowed {
			return errors.New("knowledge runtime table privileges invalid")
		}
		for _, column := range strings.Split(columns.insert, ",") {
			if err := db.WithContext(ctx).Raw("SELECT has_column_privilege(current_user,?,?,'INSERT')", "public."+table, column).Scan(&allowed).Error; err != nil || !allowed {
				return errors.New("knowledge runtime insert grants incomplete")
			}
		}
		var names []string
		if err := db.WithContext(ctx).Raw("SELECT column_name FROM information_schema.columns WHERE table_schema='public' AND table_name=?", table).Scan(&names).Error; err != nil {
			return err
		}
		for _, column := range names {
			want := strings.Contains(","+columns.update+",", ","+column+",")
			if err := db.WithContext(ctx).Raw("SELECT has_column_privilege(current_user,?,?,'UPDATE')", "public."+table, column).Scan(&allowed).Error; err != nil || allowed != want {
				return errors.New("knowledge runtime update grants invalid")
			}
		}
	}
	return nil
}

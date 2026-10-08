package ecoservices

import (
	"context"
	"errors"
	"gorm.io/gorm"
	e "task-processor/internal/ecoservices"
	"time"
)

var runtimeTables = map[string]bool{"ecoservices_applications": true, "ecoservices_listings": true, "ecoservices_requests": true, "ecoservices_financial_commands": true, "ecoservices_files": true, "ecoservices_operations": false, "ecoservices_versions": false, "ecoservices_merchant_bindings": false}

// NewRepository consumes an already provisioned, dedicated owner pool. It does
// not install tables or borrow a shared/default business database.
func NewRepository(ctx context.Context, db *gorm.DB) (*Repository, error) {
	if db == nil || db.Dialector.Name() != "postgres" {
		return nil, e.ErrUnavailable
	}
	if err := VerifyRuntime(ctx, db); err != nil {
		return nil, err
	}
	return &Repository{db: db}, nil
}
func GrantRuntime(ctx context.Context, db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return e.ErrUnavailable
	}
	for _, statement := range []string{"DO $$ BEGIN EXECUTE format('GRANT CONNECT ON DATABASE %I TO ecoservices_runtime',current_database()); END $$", "GRANT USAGE ON SCHEMA public TO ecoservices_runtime"} {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			return err
		}
	}
	for table, mutable := range runtimeTables {
		privileges := "SELECT,INSERT"
		if mutable {
			privileges += ",UPDATE"
		}
		if err := db.WithContext(ctx).Exec("GRANT " + privileges + " ON public." + table + " TO ecoservices_runtime").Error; err != nil {
			return err
		}
	}
	return nil
}
func VerifyRuntime(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return e.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var role string
	var unsafe bool
	if err := db.WithContext(ctx).Raw("SELECT current_user").Scan(&role).Error; err != nil || role != "ecoservices_runtime" {
		return errors.New("ecoservices owner runtime role is required")
	}
	if err := db.WithContext(ctx).Raw(`SELECT rolsuper OR rolcreaterole OR rolcreatedb OR rolreplication OR rolbypassrls OR has_schema_privilege(current_user,'public','CREATE') OR has_database_privilege(current_user,current_database(),'CREATE,TEMP') OR EXISTS(SELECT 1 FROM pg_auth_members WHERE member=(SELECT oid FROM pg_roles WHERE rolname=current_user)) OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND pg_has_role(current_user,c.relowner,'MEMBER')) FROM pg_roles WHERE rolname=current_user`).Scan(&unsafe).Error; err != nil || unsafe {
		return e.ErrUnavailable
	}
	var tables, immutable []string
	for table, mutable := range runtimeTables {
		var valid bool
		if err := db.WithContext(ctx).Raw(`SELECT has_table_privilege(current_user,?,'SELECT') AND has_table_privilege(current_user,?,'INSERT') AND has_table_privilege(current_user,?,'UPDATE')=? AND NOT has_table_privilege(current_user,?,'DELETE,TRUNCATE,REFERENCES,TRIGGER')`, "public."+table, "public."+table, "public."+table, mutable, "public."+table).Scan(&valid).Error; err != nil || !valid {
			return e.ErrUnavailable
		}
		tables = append(tables, table)
		if !mutable {
			immutable = append(immutable, table)
		}
	}
	var invalid int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('r','p') AND (has_any_column_privilege(current_user,c.oid,'REFERENCES') OR (c.relname NOT IN ? AND (has_any_column_privilege(current_user,c.oid,'INSERT,UPDATE') OR has_table_privilege(current_user,c.oid,'DELETE,TRUNCATE,TRIGGER'))) OR (c.relname IN ? AND has_any_column_privilege(current_user,c.oid,'UPDATE')))`, tables, immutable).Scan(&invalid).Error; err != nil || invalid != 0 {
		return e.ErrUnavailable
	}
	return nil
}

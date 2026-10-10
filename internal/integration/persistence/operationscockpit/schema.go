package operationscockpitpersistence

import (
	"context"
	_ "embed"
	"regexp"
	"time"

	"gorm.io/gorm"
	c "task-processor/internal/operationscockpit"
)

//go:embed schema.sql
var schemaSQL string
var roleName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
var tables = map[string]bool{"facts": true, "fact_versions": false, "goal_heads": true, "goal_versions": false, "commands": false}

// Install is explicit empty-schema provisioning by the current Store schema
// owner. The shared installer enforces an empty Store instance; serving never
// calls this function and no migration is admitted.
func Install(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return c.ErrUnavailable
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return tx.Exec(schemaSQL).Error })
}

func GrantRuntime(ctx context.Context, db *gorm.DB, role string) error {
	if ctx == nil || db == nil || !roleName.MatchString(role) {
		return c.ErrInvalid
	}
	if err := db.WithContext(ctx).Exec("GRANT USAGE ON SCHEMA operations_cockpit TO " + role).Error; err != nil {
		return err
	}
	for table, mutable := range tables {
		privileges := "SELECT,INSERT"
		if mutable {
			privileges += ",UPDATE"
		}
		if err := db.WithContext(ctx).Exec("GRANT " + privileges + " ON operations_cockpit." + table + " TO " + role).Error; err != nil {
			return err
		}
	}
	return nil
}

// VerifyRuntime complements the common Store capability allowlist/preflight;
// it cannot admit this schema when that shared feature flag is disabled.
func VerifyRuntime(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return c.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var safe bool
	err := db.WithContext(ctx).Raw(`SELECT NOT(rolsuper OR rolcreaterole OR rolcreatedb OR rolreplication OR rolbypassrls) AND current_schemas(false)=ARRAY['public']::name[] AND has_schema_privilege(current_user,'operations_cockpit','USAGE') AND NOT has_schema_privilege(current_user,'operations_cockpit','CREATE') FROM pg_roles WHERE rolname=current_user`).Scan(&safe).Error
	if err != nil || !safe {
		return c.ErrUnavailable
	}
	for table, mutable := range tables {
		name := "operations_cockpit." + table
		var valid bool
		err = db.WithContext(ctx).Raw(`SELECT has_table_privilege(current_user,?,'SELECT') AND has_table_privilege(current_user,?,'INSERT') AND has_table_privilege(current_user,?,'UPDATE')=? AND NOT has_table_privilege(current_user,?,'DELETE,TRUNCATE,REFERENCES,TRIGGER') AND NOT pg_has_role(current_user,(SELECT relowner FROM pg_class WHERE oid=?::regclass),'MEMBER')`, name, name, name, mutable, name, name).Scan(&valid).Error
		if err != nil || !valid {
			return c.ErrUnavailable
		}
	}
	var count int64
	if err := db.WithContext(ctx).Raw(`SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='operations_cockpit' AND c.relkind IN('r','p')`).Scan(&count).Error; err != nil || count != int64(len(tables)) {
		return c.ErrUnavailable
	}
	return nil
}

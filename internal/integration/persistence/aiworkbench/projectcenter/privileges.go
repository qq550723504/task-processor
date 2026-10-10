package projectcenterpersistence

import (
	"context"
	"gorm.io/gorm"
	"strings"
	pc "task-processor/internal/aiworkbench/projectcenter"
)

const RuntimeRole = "ai_projects_runtime"

var updates = map[string][]string{
	"projects":   {"title", "goal", "kind", "store_id", "due_date", "archived", "revision", "updated_at"},
	"references": {"active"}, "templates": {"archived", "revision"}, "visits": {"last_visited_at"},
}
var tables = []string{"projects", "references", "visits", "templates", "commands", "audit"}

func identitySafe(db *gorm.DB) error {
	var unsafe bool
	e := db.Raw(`SELECT rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls
 OR EXISTS(SELECT 1 FROM pg_auth_members WHERE member=r.oid)
 OR EXISTS(SELECT 1 FROM pg_namespace WHERE nspname='ai_workbench_projects' AND nspowner=r.oid)
 OR EXISTS(SELECT 1 FROM pg_class WHERE relnamespace='ai_workbench_projects'::regnamespace AND relowner=r.oid)
 FROM pg_roles r WHERE rolname=?`, RuntimeRole).Scan(&unsafe).Error
	if e != nil || unsafe {
		return pc.ErrUnavailable
	}
	e = db.Raw(`SELECT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname<>'ai_workbench_projects' AND n.nspname<>'information_schema' AND n.nspname NOT LIKE 'pg_%'
 AND ((c.relkind IN ('r','p','v','m','f') AND (has_table_privilege(?,c.oid,'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER') OR has_any_column_privilege(?,c.oid,'SELECT,INSERT,UPDATE,REFERENCES')))
 OR (c.relkind='S' AND has_sequence_privilege(?,c.oid,'USAGE,SELECT,UPDATE'))))`, RuntimeRole, RuntimeRole, RuntimeRole).Scan(&unsafe).Error
	if e != nil || unsafe {
		return pc.ErrUnavailable
	}
	return nil
}
func privileges(db *gorm.DB) error {
	if e := identitySafe(db); e != nil {
		return e
	}
	var ok, unsafe bool
	if e := db.Raw("SELECT has_schema_privilege(?,'ai_workbench_projects','USAGE') AND NOT has_schema_privilege(?,'ai_workbench_projects','CREATE')", RuntimeRole, RuntimeRole).Scan(&ok).Error; e != nil || !ok {
		return pc.ErrUnavailable
	}
	for _, table := range tables {
		name := "ai_workbench_projects." + table
		if e := db.Raw("SELECT has_table_privilege(?,?,'SELECT') AND has_table_privilege(?,?,'INSERT')", RuntimeRole, name, RuntimeRole, name).Scan(&ok).Error; e != nil || !ok {
			return pc.ErrUnavailable
		}
		if e := db.Raw("SELECT has_table_privilege(?,?,'DELETE,TRUNCATE,REFERENCES,TRIGGER')", RuntimeRole, name).Scan(&unsafe).Error; e != nil || unsafe {
			return pc.ErrUnavailable
		}
		allowed := updates[table]
		where := ""
		if len(allowed) > 0 {
			where = " AND attname NOT IN ('" + strings.Join(allowed, "','") + "')"
		}
		if e := db.Raw("SELECT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid=?::regclass AND attnum>0 AND NOT attisdropped"+where+" AND has_column_privilege(?,attrelid,attname,'UPDATE'))", name, RuntimeRole).Scan(&unsafe).Error; e != nil || unsafe {
			return pc.ErrUnavailable
		}
		for _, column := range allowed {
			if e := db.Raw("SELECT has_column_privilege(?,?,?,'UPDATE')", RuntimeRole, name, column).Scan(&ok).Error; e != nil || !ok {
				return pc.ErrUnavailable
			}
		}
	}
	return nil
}
func GrantRuntime(db *gorm.DB) error {
	if db == nil || db.Dialector.Name() != "postgres" {
		return pc.ErrInvalid
	}
	if e := identitySafe(db); e != nil {
		return e
	}
	statements := []string{"REVOKE ALL ON SCHEMA ai_workbench_projects FROM ai_projects_runtime", "REVOKE ALL ON ALL TABLES IN SCHEMA ai_workbench_projects FROM ai_projects_runtime", "GRANT USAGE ON SCHEMA ai_workbench_projects TO ai_projects_runtime", "GRANT SELECT,INSERT ON ALL TABLES IN SCHEMA ai_workbench_projects TO ai_projects_runtime"}
	for table, columns := range updates {
		statements = append(statements, "GRANT UPDATE ("+strings.Join(columns, ",")+") ON ai_workbench_projects."+table+" TO ai_projects_runtime")
	}
	for _, sql := range statements {
		if e := db.Exec(sql).Error; e != nil {
			return e
		}
	}
	return privileges(db)
}
func VerifySchema(ctx context.Context, db *gorm.DB) error {
	if db == nil || ctx == nil || db.Dialector.Name() != "postgres" {
		return pc.ErrUnavailable
	}
	db = db.WithContext(ctx)
	var role string
	if e := db.Raw("SELECT current_user").Scan(&role).Error; e != nil || role != RuntimeRole {
		return pc.ErrUnavailable
	}
	return privileges(db)
}

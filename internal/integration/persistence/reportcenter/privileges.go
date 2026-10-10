package reportcenterpersistence

import (
	"gorm.io/gorm"
	"regexp"
	rc "task-processor/internal/reportcenter"
)

var roleName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

func roleSafe(db *gorm.DB, role string) error {
	var safe bool
	e := db.Raw(`SELECT rolcanlogin AND NOT (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls)
 AND NOT EXISTS(SELECT 1 FROM pg_auth_members WHERE member=r.oid OR roleid=r.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspowner=r.oid)
 AND NOT has_database_privilege(r.oid,current_database(),'CREATE')
 AND NOT has_schema_privilege(r.oid,'public','CREATE')
 AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname NOT IN ('report_center','information_schema') AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
 AND ((c.relkind IN ('r','p','v','m','f') AND (has_table_privilege(r.oid,c.oid,'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER') OR has_any_column_privilege(r.oid,c.oid,'SELECT,INSERT,UPDATE,REFERENCES')))
 OR (c.relkind='S' AND has_sequence_privilege(r.oid,c.oid,'SELECT,USAGE,UPDATE'))))
 FROM pg_roles r WHERE r.rolname=?`, role).Scan(&safe).Error
	if e != nil || !safe {
		return rc.ErrUnavailable
	}
	return nil
}

// GrantRuntime is explicit empty-installation work using owner credentials.
// It never creates credentials or operates on any original business database.
func GrantRuntime(db *gorm.DB, role string) error {
	if db == nil || db.Dialector.Name() != "postgres" || !roleName.MatchString(role) || roleSafe(db, role) != nil {
		return rc.ErrInvalid
	}
	quoted := `"` + role + `"`
	return db.Transaction(func(tx *gorm.DB) error {
		for _, q := range []string{`REVOKE ALL ON SCHEMA report_center FROM ` + quoted, `REVOKE ALL ON ALL TABLES IN SCHEMA report_center FROM ` + quoted, `GRANT USAGE ON SCHEMA report_center TO ` + quoted, `GRANT SELECT,INSERT ON report_center.saved_reports,report_center.favorites,report_center.commands TO ` + quoted, `GRANT UPDATE(favorite,updated_at) ON report_center.favorites TO ` + quoted} {
			if e := tx.Exec(q).Error; e != nil {
				return e
			}
		}
		return nil
	})
}

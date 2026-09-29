package agentconfigpersistence

import (
	"gorm.io/gorm"
	"regexp"
	"task-processor/internal/agentconfig"
)

// GrantRuntime is initializer-only and does not create or elevate a role.
func GrantRuntime(db *gorm.DB, role string) error {
	if db == nil || !regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`).MatchString(role) {
		return agentconfig.ErrInvalid
	}
	var exists bool
	if e := db.Raw("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=?)", role).Scan(&exists).Error; e != nil {
		return e
	}
	if !exists {
		return agentconfig.ErrNotFound
	}
	var unsafe bool
	if e := db.Raw(`SELECT rolsuper OR rolcreaterole OR rolbypassrls OR pg_has_role(oid,'agent_configuration_owner','MEMBER') FROM pg_roles WHERE rolname=?`, role).Scan(&unsafe).Error; e != nil {
		return e
	}
	if unsafe {
		return agentconfig.ErrInvalid
	}
	quoted := `"` + role + `"`
	for _, sql := range []string{
		"REVOKE ALL ON SCHEMA agent_configuration FROM PUBLIC",
		"REVOKE ALL ON SCHEMA agent_configuration FROM " + quoted,
		"REVOKE ALL ON ALL TABLES IN SCHEMA agent_configuration FROM PUBLIC",
		"REVOKE ALL ON ALL TABLES IN SCHEMA agent_configuration FROM " + quoted,
		"GRANT USAGE ON SCHEMA agent_configuration TO " + quoted,
		"GRANT SELECT, INSERT, UPDATE ON agent_configuration.organization_agents, agent_configuration.templates TO " + quoted,
		"GRANT SELECT, INSERT ON agent_configuration.template_revisions, agent_configuration.start_snapshots, agent_configuration.commands TO " + quoted,
		"GRANT UPDATE (before_revision, after_revision, receipt, template_id) ON agent_configuration.commands TO " + quoted,
	} {
		if e := db.Exec(sql).Error; e != nil {
			return e
		}
	}
	// Revocation of direct table grants does not remove inherited or column
	// grants. Refuse unsafe effective authority instead of modifying other roles.
	if e := db.Raw("SELECT has_schema_privilege(?,'agent_configuration','CREATE')", role).Scan(&unsafe).Error; e != nil {
		return e
	}
	if unsafe {
		return agentconfig.ErrInvalid
	}
	for _, table := range []string{"organization_agents", "templates", "template_revisions", "start_snapshots", "commands"} {
		if e := db.Raw("SELECT has_table_privilege(?,?,'DELETE,TRUNCATE')", role, "agent_configuration."+table).Scan(&unsafe).Error; e != nil {
			return e
		}
		if unsafe {
			return agentconfig.ErrInvalid
		}
	}
	for _, table := range []string{"template_revisions", "start_snapshots"} {
		if e := db.Raw("SELECT has_any_column_privilege(?,?,'UPDATE')", role, "agent_configuration."+table).Scan(&unsafe).Error; e != nil {
			return e
		}
		if unsafe {
			return agentconfig.ErrInvalid
		}
	}
	if e := db.Raw(`SELECT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='agent_configuration.commands'::regclass AND attnum>0 AND NOT attisdropped AND attname NOT IN ('before_revision','after_revision','receipt','template_id') AND has_column_privilege(?,attrelid,attname,'UPDATE'))`, role).Scan(&unsafe).Error; e != nil {
		return e
	}
	if unsafe {
		return agentconfig.ErrInvalid
	}
	return nil
}

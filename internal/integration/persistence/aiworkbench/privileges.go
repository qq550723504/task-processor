package aiworkbenchpersistence

import (
	"regexp"

	"gorm.io/gorm"

	"task-processor/internal/aiworkbench"
)

var roleName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// Any membership in either direction can grant inherited access or SET ROLE
// access across the Workbench and another persistence owner.
func hasRoleMembership(db *gorm.DB, role string) (bool, error) {
	var member bool
	err := db.Raw(`SELECT EXISTS (
		SELECT 1 FROM pg_auth_members memberships
		JOIN pg_roles target ON target.oid = memberships.member OR target.oid = memberships.roleid
		WHERE target.rolname = ?)`, role).Scan(&member).Error
	return member, err
}

// Check effective relation privileges, including direct, PUBLIC, and column
// grants. Membership checks alone cannot detect a standalone cross-owner grant.
func hasCrossOwnerRelationPrivilege(db *gorm.DB, role string) (bool, error) {
	var granted bool
	err := db.Raw(`SELECT EXISTS (
		SELECT 1 FROM pg_class relation
		JOIN pg_namespace namespace ON namespace.oid = relation.relnamespace
		WHERE namespace.nspname <> 'ai_workbench'
		  AND namespace.nspname <> 'information_schema'
		  AND namespace.nspname NOT LIKE 'pg\_%' ESCAPE '\'
		  AND (
		    (relation.relkind IN ('r','p','v','m','f') AND (
		      has_table_privilege(?, relation.oid, 'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')
		      OR has_any_column_privilege(?, relation.oid, 'SELECT,INSERT,UPDATE,REFERENCES')
		    ))
		    OR (relation.relkind = 'S' AND has_sequence_privilege(?, relation.oid, 'USAGE,SELECT,UPDATE'))
		  )
	)`, role, role, role).Scan(&granted).Error
	return granted, err
}

func validateRuntimeRoleIdentity(db *gorm.DB, role string) error {
	var unsafe bool
	if err := db.Raw(`SELECT rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls FROM pg_roles WHERE rolname=?`, role).Scan(&unsafe).Error; err != nil || unsafe {
		return aiworkbench.ErrInvalid
	}
	if err := db.Raw(`SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname='ai_workbench' AND pg_get_userbyid(nspowner)=?) OR EXISTS(SELECT 1 FROM pg_class WHERE relnamespace='ai_workbench'::regnamespace AND pg_get_userbyid(relowner)=?)`, role, role).Scan(&unsafe).Error; err != nil || unsafe {
		return aiworkbench.ErrInvalid
	}
	return nil
}

// The initializer and serving startup use the same bounds. A later grant must
// not silently turn the runtime login into a schema owner or mutation authority.
func validateBoundedWorkbenchPrivileges(db *gorm.DB, role string) error {
	var unsafe bool
	if err := db.Raw("SELECT has_schema_privilege(?,'ai_workbench','CREATE')", role).Scan(&unsafe).Error; err != nil || unsafe {
		return aiworkbench.ErrInvalid
	}
	for _, table := range []string{"conversations", "metadata_audit", "messages", "commands", "execution_proposals", "business_tasks", "task_action_receipts"} {
		name := "ai_workbench." + table
		if err := db.Raw("SELECT has_table_privilege(?,?,'DELETE,TRUNCATE,REFERENCES,TRIGGER')", role, name).Scan(&unsafe).Error; err != nil || unsafe {
			return aiworkbench.ErrInvalid
		}
		if table != "conversations" && table != "commands" && table != "task_action_receipts" {
			if err := db.Raw("SELECT has_any_column_privilege(?,?,'UPDATE')", role, name).Scan(&unsafe).Error; err != nil || unsafe {
				return aiworkbench.ErrInvalid
			}
		}
	}
	for table, allowed := range map[string]string{
		"conversations":        "'title','favorite','lifecycle','metadata_revision','next_sequence','updated_at'",
		"commands":             "'state','assistant_message_id','proposal_id','terminal_digest','mode','committed_at'",
		"task_action_receipts": "'state','error_code','finished_at'",
	} {
		query := `SELECT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='ai_workbench.` + table + `'::regclass AND attnum>0 AND NOT attisdropped AND attname NOT IN (` + allowed + `) AND has_column_privilege(?,attrelid,attname,'UPDATE'))`
		if err := db.Raw(query, role).Scan(&unsafe).Error; err != nil || unsafe {
			return aiworkbench.ErrInvalid
		}
	}
	return nil
}

// GrantRuntime is initializer-only. The caller creates the restricted role
// separately and uses a privileged, private schema connection for this step.
func GrantRuntime(db *gorm.DB, role string) error {
	if db == nil || db.Dialector.Name() != "postgres" || !roleName.MatchString(role) {
		return aiworkbench.ErrInvalid
	}
	var exists bool
	if err := db.Raw("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=?)", role).Scan(&exists).Error; err != nil {
		return err
	}
	if !exists {
		return aiworkbench.ErrNotFound
	}
	member, err := hasRoleMembership(db, role)
	if err != nil || member {
		return aiworkbench.ErrInvalid
	}
	if err := validateRuntimeRoleIdentity(db, role); err != nil {
		return err
	}
	unsafe, err := hasCrossOwnerRelationPrivilege(db, role)
	if err != nil || unsafe {
		return aiworkbench.ErrInvalid
	}
	quoted := `"` + role + `"`
	statements := []string{
		"REVOKE ALL ON SCHEMA ai_workbench FROM PUBLIC",
		"REVOKE ALL ON SCHEMA ai_workbench FROM " + quoted,
		"REVOKE ALL ON ALL TABLES IN SCHEMA ai_workbench FROM PUBLIC",
		"REVOKE ALL ON ALL TABLES IN SCHEMA ai_workbench FROM " + quoted,
		"GRANT USAGE ON SCHEMA ai_workbench TO " + quoted,
		"GRANT SELECT, INSERT ON ai_workbench.conversations, ai_workbench.metadata_audit, ai_workbench.messages, ai_workbench.commands, ai_workbench.execution_proposals, ai_workbench.business_tasks, ai_workbench.task_action_receipts TO " + quoted,
		"GRANT UPDATE (title, favorite, lifecycle, metadata_revision, next_sequence, updated_at) ON ai_workbench.conversations TO " + quoted,
		"GRANT UPDATE (state, assistant_message_id, proposal_id, terminal_digest, mode, committed_at) ON ai_workbench.commands TO " + quoted,
		"GRANT UPDATE (state, error_code, finished_at) ON ai_workbench.task_action_receipts TO " + quoted,
	}
	for _, sql := range statements {
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}
	return validateBoundedWorkbenchPrivileges(db, role)
}

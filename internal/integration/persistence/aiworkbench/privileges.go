package aiworkbenchpersistence

import (
	"regexp"

	"gorm.io/gorm"

	"task-processor/internal/aiworkbench"
)

var roleName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

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
	var unsafe bool
	if err := db.Raw(`SELECT rolsuper OR rolcreaterole OR rolbypassrls FROM pg_roles WHERE rolname=?`, role).Scan(&unsafe).Error; err != nil {
		return err
	}
	if unsafe {
		return aiworkbench.ErrInvalid
	}
	if err := db.Raw(`SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname='ai_workbench' AND pg_get_userbyid(nspowner)=?) OR EXISTS(SELECT 1 FROM pg_class WHERE relnamespace='ai_workbench'::regnamespace AND pg_get_userbyid(relowner)=?)`, role, role).Scan(&unsafe).Error; err != nil || unsafe {
		return aiworkbench.ErrInvalid
	}
	quoted := `"` + role + `"`
	statements := []string{
		"REVOKE ALL ON SCHEMA ai_workbench FROM PUBLIC",
		"REVOKE ALL ON SCHEMA ai_workbench FROM " + quoted,
		"REVOKE ALL ON ALL TABLES IN SCHEMA ai_workbench FROM PUBLIC",
		"REVOKE ALL ON ALL TABLES IN SCHEMA ai_workbench FROM " + quoted,
		"GRANT USAGE ON SCHEMA ai_workbench TO " + quoted,
		"GRANT SELECT, INSERT ON ai_workbench.conversations, ai_workbench.metadata_audit, ai_workbench.messages, ai_workbench.commands, ai_workbench.execution_proposals, ai_workbench.business_tasks TO " + quoted,
		"GRANT UPDATE (title, favorite, lifecycle, metadata_revision, next_sequence, updated_at) ON ai_workbench.conversations TO " + quoted,
		"GRANT UPDATE (state, assistant_message_id, proposal_id, terminal_digest, mode, committed_at) ON ai_workbench.commands TO " + quoted,
	}
	for _, sql := range statements {
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}
	if err := db.Raw("SELECT has_schema_privilege(?,'ai_workbench','CREATE')", role).Scan(&unsafe).Error; err != nil || unsafe {
		return aiworkbench.ErrInvalid
	}
	for _, table := range []string{"conversations", "metadata_audit", "messages", "commands", "execution_proposals", "business_tasks"} {
		name := "ai_workbench." + table
		if err := db.Raw("SELECT has_table_privilege(?,?,'DELETE,TRUNCATE')", role, name).Scan(&unsafe).Error; err != nil || unsafe {
			return aiworkbench.ErrInvalid
		}
		if table != "conversations" && table != "commands" {
			if err := db.Raw("SELECT has_any_column_privilege(?,?,'UPDATE')", role, name).Scan(&unsafe).Error; err != nil || unsafe {
				return aiworkbench.ErrInvalid
			}
		}
	}
	for table, allowed := range map[string]string{
		"conversations": "'title','favorite','lifecycle','metadata_revision','next_sequence','updated_at'",
		"commands":      "'state','assistant_message_id','proposal_id','terminal_digest','mode','committed_at'",
	} {
		query := `SELECT EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid='ai_workbench.` + table + `'::regclass AND attnum>0 AND NOT attisdropped AND attname NOT IN (` + allowed + `) AND has_column_privilege(?,attrelid,attname,'UPDATE'))`
		if err := db.Raw(query, role).Scan(&unsafe).Error; err != nil || unsafe {
			return aiworkbench.ErrInvalid
		}
	}
	return nil
}

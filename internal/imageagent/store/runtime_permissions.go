package store

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"gorm.io/gorm"
)

const OrganizationRuntimeRole = "image_agent_runtime"

var errOrganizationRuntimePermissions = errors.New("organization image agent runtime permissions unavailable")
var organizationRuntimeRoleName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// API Prepare/Confirm/Cancel uses this role. Only the confirmation/status
// columns are writable. Candidate verification reads original provider facts
// and materialization; their writes stay with the worker and existing owners.
var organizationRuntimeTables = []struct{ name, privileges string }{
	{"image_agent_v2_runs", "SELECT,INSERT"},
	{"image_agent_v2_plans", "INSERT"},
	{"image_agent_v2_slots", "INSERT"},
	{"image_agent_v2_events", "INSERT"},
	{"image_agent_v2_asset_catalog", "INSERT"},
	{"image_agent_v2_asset_catalog_manifests", "INSERT"},
	{"image_agent_v2_projection_snapshots", "SELECT,INSERT"},
	{"image_agent_v2_projection_commits", "SELECT,INSERT"},
	{"image_agent_v3_slot_external_effects", "SELECT"},
	{"product_approval_receipts", "SELECT"},
	{"product_approved_assets", "SELECT"},
}

var organizationRuntimeColumnGrants = []struct{ name, columns string }{
	{"image_agent_v2_runs", "admission_json,status,current_node,active_plan_revision,block_json,version,updated_at"},
	{"image_agent_v2_projection_snapshots", "version,snapshot_json,updated_at"},
}

const organizationRuntimePermissionQuery = `WITH admitted(table_name,privilege) AS (VALUES
 ('image_agent_v2_runs','SELECT'),('image_agent_v2_runs','INSERT'),
 ('image_agent_v2_plans','INSERT'),
 ('image_agent_v2_slots','INSERT'),
 ('image_agent_v2_events','INSERT'),
 ('image_agent_v2_asset_catalog','INSERT'),
 ('image_agent_v2_asset_catalog_manifests','INSERT'),
 ('image_agent_v2_projection_snapshots','SELECT'),('image_agent_v2_projection_snapshots','INSERT'),
 ('image_agent_v2_projection_commits','SELECT'),('image_agent_v2_projection_commits','INSERT'),
 ('image_agent_v3_slot_external_effects','SELECT'),
 ('product_approval_receipts','SELECT'),('product_approved_assets','SELECT')),
 admitted_columns(table_name,column_name) AS (VALUES
 ('image_agent_v2_runs','admission_json'),('image_agent_v2_runs','status'),
 ('image_agent_v2_runs','current_node'),('image_agent_v2_runs','active_plan_revision'),
 ('image_agent_v2_runs','block_json'),('image_agent_v2_runs','version'),('image_agent_v2_runs','updated_at'),
 ('image_agent_v2_projection_snapshots','version'),('image_agent_v2_projection_snapshots','snapshot_json'),
 ('image_agent_v2_projection_snapshots','updated_at'))
 SELECT current_user::text AS role_name,
 session_user=current_user
 AND current_schemas(false)=ARRAY['public']::name[]
 AND pg_my_temp_schema()=0
 AND has_database_privilege(current_user,current_database(),'CONNECT')
 AND has_schema_privilege(current_user,'public','USAGE')
 AND EXISTS(SELECT 1 FROM pg_roles WHERE rolname=current_user AND rolcanlogin
   AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls)
 AND NOT EXISTS(SELECT 1 FROM admitted WHERE NOT coalesce(has_table_privilege(current_user,format('public.%I',table_name),privilege),false))
 AND NOT EXISTS(SELECT 1 FROM admitted WHERE to_regclass(format('public.%I',table_name)) IS NULL)
 AND NOT EXISTS(SELECT 1 FROM admitted_columns WHERE NOT coalesce(has_column_privilege(current_user,format('public.%I',table_name),column_name,'UPDATE'),false))
 AS required_privileges,
 has_database_privilege(current_user,current_database(),'CREATE')
 OR EXISTS(SELECT 1 FROM pg_auth_members WHERE member=(SELECT oid FROM pg_roles WHERE rolname=current_user))
 OR EXISTS(SELECT 1 FROM pg_namespace n WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'
   AND (has_schema_privilege(current_user,n.oid,'CREATE') OR (n.nspname<>'public' AND has_schema_privilege(current_user,n.oid,'USAGE'))))
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   CROSS JOIN (VALUES('SELECT'),('INSERT'),('UPDATE'),('DELETE'),('TRUNCATE'),('REFERENCES'),('TRIGGER')) p(privilege)
   WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema' AND c.relkind IN ('r','p','v','m','f')
   AND CASE WHEN p.privilege='UPDATE' THEN EXISTS(SELECT 1 FROM pg_attribute att WHERE att.attrelid=c.oid AND att.attnum>0 AND NOT att.attisdropped AND has_column_privilege(current_user,c.oid,att.attnum,'UPDATE')
     AND NOT EXISTS(SELECT 1 FROM admitted_columns ac WHERE n.nspname='public' AND ac.table_name=c.relname AND ac.column_name=att.attname))
     WHEN p.privilege IN ('SELECT','INSERT','REFERENCES')
     THEN has_any_column_privilege(current_user,c.oid,p.privilege)
     ELSE has_table_privilege(current_user,c.oid,p.privilege) END
   AND NOT EXISTS(SELECT 1 FROM admitted a WHERE n.nspname='public' AND a.table_name=c.relname AND a.privilege=p.privilege))
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE n.nspname='public' AND c.relname IN (SELECT table_name FROM admitted)
   AND (c.relkind<>'r' OR c.relrowsecurity OR c.relforcerowsecurity OR c.relowner=(SELECT oid FROM pg_roles WHERE rolname=current_user)))
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE c.relkind='S' AND n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'
   AND has_sequence_privilege(current_user,c.oid,'USAGE,SELECT,UPDATE')) AS forbidden_privileges`

// GrantOrganizationRuntimePermissions is explicit maintenance against the
// already-owned ImageAgent database. It never creates a login, migrates schema,
// grants Product/Catalog access, or runs during normal current-app startup.
func GrantOrganizationRuntimePermissions(ctx context.Context, db *gorm.DB) error {
	return grantOrganizationRuntimePermissions(ctx, db, OrganizationRuntimeRole)
}

func grantOrganizationRuntimePermissions(ctx context.Context, db *gorm.DB, role string) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" || !organizationRuntimeRoleName.MatchString(role) {
		return errOrganizationRuntimePermissions
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var roleSafe bool
		if err := tx.Raw(`SELECT EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolname=? AND r.rolcanlogin
 AND NOT r.rolsuper AND NOT r.rolcreatedb AND NOT r.rolcreaterole AND NOT r.rolreplication AND NOT r.rolbypassrls
			AND NOT EXISTS(SELECT 1 FROM pg_auth_members m WHERE m.member=r.oid))`, role).Scan(&roleSafe).Error; err != nil || !roleSafe {
			return errOrganizationRuntimePermissions
		}
		var database string
		if err := tx.Raw("SELECT current_database()").Scan(&database).Error; err != nil || database == "" || database == "postgres" || database == "template0" || database == "template1" {
			return errOrganizationRuntimePermissions
		}
		quoted := `"` + strings.ReplaceAll(database, `"`, `""`) + `"`
		quotedRole := `"` + role + `"`
		statements := []string{
			"REVOKE ALL PRIVILEGES ON DATABASE " + quoted + " FROM " + quotedRole,
			"GRANT CONNECT ON DATABASE " + quoted + " TO " + quotedRole,
			"REVOKE ALL PRIVILEGES ON SCHEMA public FROM " + quotedRole,
			"GRANT USAGE ON SCHEMA public TO " + quotedRole,
		}
		for _, table := range organizationRuntimeTables {
			statements = append(statements, "REVOKE ALL PRIVILEGES ON TABLE public."+table.name+" FROM "+quotedRole, "GRANT "+table.privileges+" ON TABLE public."+table.name+" TO "+quotedRole)
		}
		for _, grant := range organizationRuntimeColumnGrants {
			statements = append(statements, "GRANT UPDATE("+grant.columns+") ON TABLE public."+grant.name+" TO "+quotedRole)
		}
		for _, statement := range statements {
			if err := tx.Exec(statement).Error; err != nil {
				return errOrganizationRuntimePermissions
			}
		}
		return nil
	})
}

// VerifyOrganizationRuntimePermissions is read-only startup admission. An
// unexpected column/table/schema grant fails closed even when required grants
// are present; the current app never repairs role permissions by itself.
func VerifyOrganizationRuntimePermissions(ctx context.Context, db *gorm.DB) error {
	return verifyOrganizationRuntimePermissions(ctx, db, OrganizationRuntimeRole)
}

func verifyOrganizationRuntimePermissions(ctx context.Context, db *gorm.DB, role string) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" || !organizationRuntimeRoleName.MatchString(role) {
		return errOrganizationRuntimePermissions
	}
	pool, err := db.DB()
	if err != nil {
		return errOrganizationRuntimePermissions
	}
	maximum := pool.Stats().MaxOpenConnections
	if maximum < 1 || maximum > 8 {
		return errOrganizationRuntimePermissions
	}
	var user string
	var required, forbidden bool
	if err := pool.QueryRowContext(ctx, organizationRuntimePermissionQuery).Scan(&user, &required, &forbidden); err != nil || user != role || !required || forbidden {
		return errOrganizationRuntimePermissions
	}
	return nil
}

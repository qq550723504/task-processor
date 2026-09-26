package store

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"
)

const OrganizationWorkerRuntimeRole = "image_agent_worker_runtime"

var errOrganizationWorkerRuntimePermissions = errors.New("organization image agent worker runtime permissions unavailable")

// This is the current organization-v1 queue's exact owner-DB footprint.
// The old v2 slot-effect table is deliberately not admitted. Product Asset
// approval remains with its existing owner; this role only invokes it.
var organizationWorkerRuntimeTables = []struct{ name, privileges string }{
	{"ai_client_credentials", "SELECT"},
	{"ai_invocations", "SELECT,INSERT,UPDATE"},
	{"image_agent_v2_runs", "SELECT,UPDATE"},
	{"image_agent_v2_plans", "SELECT,INSERT"},
	{"image_agent_v2_slots", "SELECT,INSERT,UPDATE"},
	{"image_agent_v2_attempts", "SELECT,INSERT"},
	{"image_agent_v2_events", "SELECT,INSERT"},
	{"image_agent_v2_asset_catalog", "SELECT"},
	{"image_agent_v2_asset_catalog_manifests", "SELECT"},
	{"image_agent_v2_projection_snapshots", "SELECT,INSERT,UPDATE"},
	{"image_agent_v2_projection_commits", "SELECT,INSERT"},
	{"image_agent_v3_slot_external_effects", "SELECT,INSERT,UPDATE"},
	{"product_approved_assets", "SELECT,INSERT"},
	{"product_approval_receipts", "SELECT,INSERT"},
	{"product_approved_inventory_heads", "SELECT,INSERT,UPDATE"},
	{"product_approved_inventory_version_heads", "SELECT,INSERT,UPDATE"},
}

// GrantOrganizationWorkerRuntimePermissions is explicit owner maintenance.
// It never creates a role or runs during ordinary worker startup.
func GrantOrganizationWorkerRuntimePermissions(ctx context.Context, db *gorm.DB) error {
	return grantOrganizationWorkerRuntimePermissions(ctx, db, OrganizationWorkerRuntimeRole)
}

func grantOrganizationWorkerRuntimePermissions(ctx context.Context, db *gorm.DB, role string) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" || !organizationRuntimeRoleName.MatchString(role) {
		return errOrganizationWorkerRuntimePermissions
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var roleSafe bool
		if err := tx.Raw(`SELECT EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolname=? AND r.rolcanlogin
 AND NOT r.rolsuper AND NOT r.rolcreatedb AND NOT r.rolcreaterole AND NOT r.rolreplication AND NOT r.rolbypassrls
 AND NOT EXISTS(SELECT 1 FROM pg_auth_members m WHERE m.member=r.oid))`, role).Scan(&roleSafe).Error; err != nil || !roleSafe {
			return errOrganizationWorkerRuntimePermissions
		}
		var database string
		if err := tx.Raw("SELECT current_database()").Scan(&database).Error; err != nil || database == "" || database == "postgres" || database == "template0" || database == "template1" {
			return errOrganizationWorkerRuntimePermissions
		}
		quotedDB := `"` + strings.ReplaceAll(database, `"`, `""`) + `"`
		quotedRole := `"` + role + `"`
		statements := []string{
			"REVOKE ALL PRIVILEGES ON DATABASE " + quotedDB + " FROM " + quotedRole,
			"GRANT CONNECT ON DATABASE " + quotedDB + " TO " + quotedRole,
			"REVOKE ALL PRIVILEGES ON SCHEMA public FROM " + quotedRole,
			"GRANT USAGE ON SCHEMA public TO " + quotedRole,
		}
		for _, table := range organizationWorkerRuntimeTables {
			statements = append(statements, "REVOKE ALL PRIVILEGES ON TABLE public."+table.name+" FROM "+quotedRole, "GRANT "+table.privileges+" ON TABLE public."+table.name+" TO "+quotedRole)
		}
		for _, statement := range statements {
			if err := tx.Exec(statement).Error; err != nil {
				return errOrganizationWorkerRuntimePermissions
			}
		}
		return nil
	})
}

// VerifyOrganizationWorkerRuntimePermissions performs read-only admission
// before the worker polls Temporal or invokes a provider. Any extra table,
// column, sequence, schema or inherited role privilege fails closed.
func VerifyOrganizationWorkerRuntimePermissions(ctx context.Context, db *gorm.DB) error {
	return verifyOrganizationWorkerRuntimePermissions(ctx, db, OrganizationWorkerRuntimeRole)
}

func verifyOrganizationWorkerRuntimePermissions(ctx context.Context, db *gorm.DB, role string) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" || !organizationRuntimeRoleName.MatchString(role) {
		return errOrganizationWorkerRuntimePermissions
	}
	pool, err := db.DB()
	if err != nil || pool.Stats().MaxOpenConnections < 1 || pool.Stats().MaxOpenConnections > 8 {
		return errOrganizationWorkerRuntimePermissions
	}
	var user string
	var required, forbidden bool
	if err := pool.QueryRowContext(ctx, organizationWorkerRuntimePermissionQuery()).Scan(&user, &required, &forbidden); err != nil || user != role || !required || forbidden {
		return errOrganizationWorkerRuntimePermissions
	}
	return nil
}

func organizationWorkerRuntimePermissionQuery() string {
	var values []string
	for _, table := range organizationWorkerRuntimeTables {
		for _, privilege := range strings.Split(table.privileges, ",") {
			values = append(values, "('"+table.name+"','"+privilege+"')")
		}
	}
	return `WITH admitted(table_name,privilege) AS (VALUES ` + strings.Join(values, ",") + `)
 SELECT current_user::text,
 session_user=current_user
 AND current_schemas(false)=ARRAY['public']::name[]
 AND pg_my_temp_schema()=0
 AND has_database_privilege(current_user,current_database(),'CONNECT')
 AND has_schema_privilege(current_user,'public','USAGE')
 AND EXISTS(SELECT 1 FROM pg_roles WHERE rolname=current_user AND rolcanlogin
   AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls)
 AND NOT EXISTS(SELECT 1 FROM admitted WHERE NOT coalesce(has_table_privilege(current_user,format('public.%I',table_name),privilege),false))
 AND NOT EXISTS(SELECT 1 FROM admitted WHERE to_regclass(format('public.%I',table_name)) IS NULL),
 has_database_privilege(current_user,current_database(),'CREATE')
 OR EXISTS(SELECT 1 FROM pg_auth_members WHERE member=(SELECT oid FROM pg_roles WHERE rolname=current_user))
 OR EXISTS(SELECT 1 FROM pg_namespace n WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'
   AND (has_schema_privilege(current_user,n.oid,'CREATE') OR (n.nspname<>'public' AND has_schema_privilege(current_user,n.oid,'USAGE'))))
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   CROSS JOIN (VALUES('SELECT'),('INSERT'),('UPDATE'),('DELETE'),('TRUNCATE'),('REFERENCES'),('TRIGGER')) p(privilege)
   WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema' AND c.relkind IN ('r','p','v','m','f')
   AND CASE WHEN p.privilege IN ('SELECT','INSERT','UPDATE','REFERENCES')
     THEN has_any_column_privilege(current_user,c.oid,p.privilege)
     ELSE has_table_privilege(current_user,c.oid,p.privilege) END
   AND NOT EXISTS(SELECT 1 FROM admitted a WHERE n.nspname='public' AND a.table_name=c.relname AND a.privilege=p.privilege))
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE n.nspname='public' AND c.relname IN (SELECT table_name FROM admitted)
   AND (c.relkind<>'r' OR c.relrowsecurity OR c.relforcerowsecurity OR c.relowner=(SELECT oid FROM pg_roles WHERE rolname=current_user)))
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE c.relkind='S' AND n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'
   AND has_sequence_privilege(current_user,c.oid,'USAGE,SELECT,UPDATE'))`
}

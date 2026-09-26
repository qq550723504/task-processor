package orgresourceadapter

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

const RuntimeRole = "commercial_owner_runtime"

// The role is shared with the existing commercial owner. This check admits
// only the resource owner's required DML on its own tables, not arbitrary
// subscription/billing grants. It never grants, migrates, or changes a role.
const resourceRuntimePermissionQuery = `WITH owned(table_name,mutable) AS (VALUES
 ('saas_organization_resource_buckets',true),
 ('saas_organization_resource_operations',true),
 ('saas_organization_resource_reservations',true),
 ('saas_organization_resource_debts',true),
 ('saas_member_ai_point_limits',true),
 ('saas_member_ai_point_months',true),
 ('saas_organization_resource_source_claims',false),
 ('saas_organization_resource_events',false),
 ('saas_organization_resource_audit_logs',false))
 SELECT current_user::text,
 session_user=current_user
 AND current_schemas(false)=ARRAY['public']::name[]
 AND pg_my_temp_schema()=0
 AND has_database_privilege(current_user,current_database(),'CONNECT')
 AND has_schema_privilege(current_user,'public','USAGE')
 AND EXISTS(SELECT 1 FROM pg_roles WHERE rolname=current_user AND rolcanlogin
   AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls)
 AND NOT EXISTS(SELECT 1 FROM owned WHERE to_regclass(format('public.%I',table_name)) IS NULL)
 AND NOT EXISTS(SELECT 1 FROM owned WHERE
   NOT coalesce(has_table_privilege(current_user,format('public.%I',table_name),'SELECT'),false)
   OR NOT coalesce(has_table_privilege(current_user,format('public.%I',table_name),'INSERT'),false)
   OR (mutable AND NOT coalesce(has_table_privilege(current_user,format('public.%I',table_name),'UPDATE'),false)))
 AND has_sequence_privilege(current_user,'public.saas_organization_resource_audit_logs_id_seq','USAGE')
 AND has_sequence_privilege(current_user,'public.saas_organization_resource_audit_logs_id_seq','SELECT'),
 has_database_privilege(current_user,current_database(),'CREATE')
 OR has_schema_privilege(current_user,'public','CREATE')
 OR EXISTS(SELECT 1 FROM pg_auth_members WHERE member=(SELECT oid FROM pg_roles WHERE rolname=current_user))
 OR EXISTS(SELECT 1 FROM owned o JOIN pg_class c ON c.oid=to_regclass(format('public.%I',o.table_name))
   WHERE c.relkind<>'r' OR c.relrowsecurity OR c.relforcerowsecurity
   OR c.relowner=(SELECT oid FROM pg_roles WHERE rolname=current_user)
   OR has_table_privilege(current_user,c.oid,'DELETE,TRUNCATE,TRIGGER,REFERENCES')
   OR has_any_column_privilege(current_user,c.oid,'REFERENCES')
   OR (NOT o.mutable AND has_any_column_privilege(current_user,c.oid,'UPDATE')))
 OR has_sequence_privilege(current_user,'public.saas_organization_resource_audit_logs_id_seq','UPDATE')`

func VerifyRuntimePermissions(ctx context.Context, db *gorm.DB) error {
	invalid := errors.New("organization resource runtime permissions unavailable")
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return invalid
	}
	pool, err := db.DB()
	if err != nil {
		return invalid
	}
	var role string
	var required, forbidden bool
	if err := pool.QueryRowContext(ctx, resourceRuntimePermissionQuery).Scan(&role, &required, &forbidden); err != nil || role != RuntimeRole || !required || forbidden {
		return invalid
	}
	return nil
}

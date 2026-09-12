package membership

import (
	"context"
	"errors"
	"gorm.io/gorm"
)

// Effective PostgreSQL privileges include PUBLIC, inherited roles and columns.
// This check reads catalogs; it never probes permission by mutating data.
const runtimePermissionQuery = `SELECT current_user,
 has_database_privilege(current_user,current_database(),'CONNECT')
 AND has_schema_privilege(current_user,'public','USAGE')
 AND has_table_privilege(current_user,'public.organization_member_operations','SELECT')
 AND has_table_privilege(current_user,'public.organization_member_operations','INSERT')
 AND has_table_privilege(current_user,'public.organization_member_operations','UPDATE') AS required,
 has_database_privilege(current_user,current_database(),'CREATE')
 OR has_database_privilege(current_user,current_database(),'TEMPORARY')
 OR EXISTS (SELECT 1 FROM pg_roles WHERE rolname=current_user AND (rolsuper OR rolcreaterole OR rolcreatedb OR rolreplication OR rolbypassrls))
 OR EXISTS (SELECT 1 FROM pg_namespace WHERE nspname NOT LIKE 'pg_%' AND nspname<>'information_schema' AND has_schema_privilege(current_user,oid,'CREATE'))
 OR EXISTS (
 SELECT 1 FROM pg_class relation JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
 CROSS JOIN LATERAL aclexplode(acldefault('r',relation.relowner)) privilege
 WHERE namespace.nspname NOT LIKE 'pg_%' AND namespace.nspname<>'information_schema'
 AND relation.relkind IN ('r','p','v','m','f')
 AND CASE WHEN privilege.privilege_type IN ('SELECT','INSERT','UPDATE','REFERENCES')
 THEN has_any_column_privilege(current_user,relation.oid,privilege.privilege_type)
 ELSE has_table_privilege(current_user,relation.oid,privilege.privilege_type) END
 AND NOT (namespace.nspname='public' AND relation.relname='organization_member_operations' AND privilege.privilege_type IN ('SELECT','INSERT','UPDATE'))
 ) AS forbidden`

func VerifyRuntimePermissions(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("membership runtime database unavailable")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return errors.New("membership runtime database unavailable")
	}
	var user string
	var required, forbidden bool
	if err := sqlDB.QueryRowContext(ctx, runtimePermissionQuery).Scan(&user, &required, &forbidden); err != nil {
		return errors.New("membership runtime permission inspection failed")
	}
	if user != "organization_membership_runtime" || !required || forbidden {
		return errors.New("membership runtime permissions do not match its owner boundary")
	}
	return nil
}

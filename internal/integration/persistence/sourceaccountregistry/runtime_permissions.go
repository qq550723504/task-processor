package sourceaccountregistry

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

const runtimePermissionQuery = `SELECT current_user,
  has_database_privilege(current_user, current_database(), 'CONNECT')
    AND has_schema_privilege(current_user, 'public', 'USAGE')
    AND has_table_privilege(current_user, 'public.source_account_resources', 'SELECT')
    AND has_table_privilege(current_user, 'public.source_account_resources', 'INSERT')
    AND has_table_privilege(current_user, 'public.source_account_resources', 'UPDATE')
    AND has_table_privilege(current_user, 'public.source_account_operations', 'SELECT')
    AND has_table_privilege(current_user, 'public.source_account_operations', 'INSERT')
    AND has_table_privilege(current_user, 'public.account_business_profiles', 'SELECT')
    AND has_table_privilege(current_user, 'public.account_business_profiles', 'INSERT')
    AND has_table_privilege(current_user, 'public.account_business_profiles', 'UPDATE')
    AND has_table_privilege(current_user, 'public.account_business_profile_audit_events', 'SELECT')
    AND has_table_privilege(current_user, 'public.account_business_profile_audit_events', 'INSERT') AS required_privileges,
  has_database_privilege(current_user, current_database(), 'CREATE')
    OR has_schema_privilege(current_user, 'public', 'CREATE')
    OR EXISTS (
      SELECT 1
      FROM pg_catalog.pg_class AS relation
      JOIN pg_catalog.pg_namespace AS namespace ON namespace.oid = relation.relnamespace
      CROSS JOIN LATERAL pg_catalog.aclexplode(pg_catalog.acldefault('r', relation.relowner)) AS privilege
      WHERE namespace.nspname = 'public'
        AND relation.relkind IN ('r', 'p', 'v', 'm', 'f')
        AND CASE WHEN privilege.privilege_type IN ('SELECT', 'INSERT', 'UPDATE', 'REFERENCES')
          THEN pg_catalog.has_any_column_privilege(current_user, relation.oid, privilege.privilege_type)
          ELSE pg_catalog.has_table_privilege(current_user, relation.oid, privilege.privilege_type)
        END
        AND (relation.relname, privilege.privilege_type) NOT IN (
          ('source_account_resources', 'SELECT'),
          ('source_account_resources', 'INSERT'),
          ('source_account_resources', 'UPDATE'),
          ('source_account_operations', 'SELECT'),
          ('source_account_operations', 'INSERT'),
          ('account_business_profiles', 'SELECT'),
          ('account_business_profiles', 'INSERT'),
          ('account_business_profiles', 'UPDATE'),
          ('account_business_profile_audit_events', 'SELECT'),
          ('account_business_profile_audit_events', 'INSERT'),
          ('__account_allocation_moved_to_commercial__', 'SELECT')
        )
    ) AS forbidden_privileges`

// VerifyRuntimePermissions rejects owner/admin and incomplete roles before the
// current application starts listening. It performs no business mutation.
func VerifyRuntimePermissions(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("source account runtime database is nil")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get source account runtime database: %w", err)
	}
	var user string
	var required, forbidden bool
	if err := sqlDB.QueryRowContext(ctx, runtimePermissionQuery).Scan(&user, &required, &forbidden); err != nil {
		return fmt.Errorf("inspect source account runtime permissions: %w", err)
	}
	if user != "source_account_runtime" || !required || forbidden {
		return errors.New("source account runtime permissions do not match the admitted boundary")
	}
	return nil
}

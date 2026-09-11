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
    AND has_table_privilege(current_user, 'public.source_account_operations', 'INSERT') AS required_privileges,
  has_database_privilege(current_user, current_database(), 'CREATE')
    OR has_schema_privilege(current_user, 'public', 'CREATE')
    OR has_table_privilege(current_user, 'public.source_account_resources', 'DELETE')
    OR has_table_privilege(current_user, 'public.source_account_resources', 'TRUNCATE')
    OR has_table_privilege(current_user, 'public.source_account_resources', 'TRIGGER')
    OR has_table_privilege(current_user, 'public.source_account_operations', 'UPDATE')
    OR has_table_privilege(current_user, 'public.source_account_operations', 'DELETE')
    OR has_table_privilege(current_user, 'public.source_account_operations', 'TRUNCATE')
    OR has_table_privilege(current_user, 'public.source_account_operations', 'TRIGGER')
    OR has_table_privilege(current_user, 'public.goose_source_account_registry_version', 'SELECT')
    OR has_table_privilege(current_user, 'public.saas_tenant_subscriptions', 'SELECT')
    OR has_table_privilege(current_user, 'public.saas_tenant_subscriptions', 'INSERT')
    OR has_table_privilege(current_user, 'public.saas_tenant_subscriptions', 'UPDATE')
    OR has_table_privilege(current_user, 'public.saas_tenant_subscriptions', 'DELETE')
    OR has_table_privilege(current_user, 'public.saas_plans', 'SELECT')
    OR has_table_privilege(current_user, 'public.saas_plans', 'INSERT')
    OR has_table_privilege(current_user, 'public.saas_plans', 'UPDATE')
    OR has_table_privilege(current_user, 'public.saas_plans', 'DELETE')
    OR has_table_privilege(current_user, 'public.saas_tenant_entitlements', 'SELECT')
    OR has_table_privilege(current_user, 'public.saas_tenant_entitlements', 'INSERT')
    OR has_table_privilege(current_user, 'public.saas_tenant_entitlements', 'UPDATE')
    OR has_table_privilege(current_user, 'public.saas_tenant_entitlements', 'DELETE')
    OR has_table_privilege(current_user, 'public.saas_usage_buckets', 'SELECT')
    OR has_table_privilege(current_user, 'public.saas_usage_buckets', 'INSERT')
    OR has_table_privilege(current_user, 'public.saas_usage_buckets', 'UPDATE')
    OR has_table_privilege(current_user, 'public.saas_usage_buckets', 'DELETE') AS forbidden_privileges`

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

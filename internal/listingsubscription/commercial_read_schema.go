package listingsubscription

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// commercialReadSchemaQueries are deliberately narrow SELECT-only probes for
// the four current-owner fact tables used by ReadCommercialOverview. They
// verify both the required columns and the runtime role's SELECT permission
// without migrating, seeding, repairing, or reading business rows.
var commercialReadSchemaQueries = []string{
	`SELECT tenant_id, plan_code, status, starts_at, expires_at, updated_at FROM public.saas_tenant_subscriptions WHERE FALSE`,
	`SELECT code, name FROM public.saas_plans WHERE FALSE`,
	`SELECT tenant_id, module_code, status, starts_at, expires_at, limits, updated_at FROM public.saas_tenant_entitlements WHERE FALSE`,
	`SELECT tenant_id, module_code, period_key, metric, committed, reserved, updated_at FROM public.saas_usage_buckets WHERE FALSE`,
}

const commercialReadPermissionQuery = `SELECT current_user,
  has_database_privilege(current_user, current_database(), 'CONNECT')
    AND has_schema_privilege(current_user, 'public', 'USAGE')
    AND has_table_privilege(current_user, 'public.saas_tenant_subscriptions', 'SELECT')
    AND has_table_privilege(current_user, 'public.saas_plans', 'SELECT')
    AND has_table_privilege(current_user, 'public.saas_tenant_entitlements', 'SELECT')
    AND has_table_privilege(current_user, 'public.saas_usage_buckets', 'SELECT') AS required_privileges,
  has_database_privilege(current_user, current_database(), 'CREATE')
    OR has_schema_privilege(current_user, 'public', 'CREATE')
    OR has_table_privilege(current_user, 'public.saas_tenant_subscriptions', 'INSERT')
    OR has_table_privilege(current_user, 'public.saas_tenant_subscriptions', 'UPDATE')
    OR has_table_privilege(current_user, 'public.saas_tenant_subscriptions', 'DELETE')
    OR has_table_privilege(current_user, 'public.saas_tenant_subscriptions', 'TRUNCATE')
    OR has_table_privilege(current_user, 'public.saas_tenant_subscriptions', 'TRIGGER')
    OR has_table_privilege(current_user, 'public.saas_plans', 'INSERT')
    OR has_table_privilege(current_user, 'public.saas_plans', 'UPDATE')
    OR has_table_privilege(current_user, 'public.saas_plans', 'DELETE')
    OR has_table_privilege(current_user, 'public.saas_plans', 'TRUNCATE')
    OR has_table_privilege(current_user, 'public.saas_plans', 'TRIGGER')
    OR has_table_privilege(current_user, 'public.saas_tenant_entitlements', 'INSERT')
    OR has_table_privilege(current_user, 'public.saas_tenant_entitlements', 'UPDATE')
    OR has_table_privilege(current_user, 'public.saas_tenant_entitlements', 'DELETE')
    OR has_table_privilege(current_user, 'public.saas_tenant_entitlements', 'TRUNCATE')
    OR has_table_privilege(current_user, 'public.saas_tenant_entitlements', 'TRIGGER')
    OR has_table_privilege(current_user, 'public.saas_usage_buckets', 'INSERT')
    OR has_table_privilege(current_user, 'public.saas_usage_buckets', 'UPDATE')
    OR has_table_privilege(current_user, 'public.saas_usage_buckets', 'DELETE')
    OR has_table_privilege(current_user, 'public.saas_usage_buckets', 'TRUNCATE')
    OR has_table_privilege(current_user, 'public.saas_usage_buckets', 'TRIGGER')
    OR has_table_privilege(current_user, 'public.source_account_resources', 'SELECT')
    OR has_table_privilege(current_user, 'public.source_account_resources', 'INSERT')
    OR has_table_privilege(current_user, 'public.source_account_resources', 'UPDATE')
    OR has_table_privilege(current_user, 'public.source_account_resources', 'DELETE')
    OR has_table_privilege(current_user, 'public.source_account_operations', 'SELECT')
    OR has_table_privilege(current_user, 'public.source_account_operations', 'INSERT')
    OR has_table_privilege(current_user, 'public.source_account_operations', 'UPDATE')
    OR has_table_privilege(current_user, 'public.source_account_operations', 'DELETE') AS forbidden_privileges`

// VerifyCommercialReadSchema fails closed before the application listens when
// the read-only commercial role cannot access the exact current fact boundary.
func VerifyCommercialReadSchema(ctx context.Context, db *gorm.DB) error {
	if db == nil {
		return errors.New("commercial read database is nil")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get commercial read database: %w", err)
	}
	tx, err := sqlDB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return fmt.Errorf("begin commercial read readiness probe: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, query := range commercialReadSchemaQueries {
		rows, queryErr := tx.QueryContext(ctx, query)
		if queryErr != nil {
			return fmt.Errorf("probe commercial read schema: %w", queryErr)
		}
		if closeErr := rows.Close(); closeErr != nil {
			return fmt.Errorf("close commercial read readiness probe: %w", closeErr)
		}
	}
	var user string
	var required, forbidden bool
	if err := tx.QueryRowContext(ctx, commercialReadPermissionQuery).Scan(&user, &required, &forbidden); err != nil {
		return fmt.Errorf("inspect commercial read permissions: %w", err)
	}
	if user != "commercial_reader" || !required || forbidden {
		return errors.New("commercial read permissions do not match the admitted boundary")
	}
	return nil
}

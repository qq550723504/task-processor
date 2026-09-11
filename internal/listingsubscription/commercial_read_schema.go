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
          ('saas_tenant_subscriptions', 'SELECT'),
          ('saas_plans', 'SELECT'),
          ('saas_tenant_entitlements', 'SELECT'),
          ('saas_usage_buckets', 'SELECT')
        )
    ) AS forbidden_privileges`

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

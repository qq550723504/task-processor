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
	return nil
}

package listingsubscription

import (
	"context"

	"gorm.io/gorm"
)

const retiredSystemStudioModuleCode = "studio"

func (r *GormRepository) SyncDefaultCatalog(ctx context.Context, modules []Module, plans []PlanBundle) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := retireSystemModule(tx, retiredSystemStudioModuleCode); err != nil {
			return err
		}
		if err := upsertDefaultModules(tx, modules); err != nil {
			return err
		}
		return upsertDefaultPlans(tx, plans)
	})
}

func retireSystemModule(tx *gorm.DB, moduleCode string) error {
	// Catalog references may be retired, but usage events, outbox rows, usage
	// aggregates, and audit records are immutable evidence. Deleting them here
	// would lose committed billing history and strand pending delivery.
	deletions := []struct {
		model any
		query string
		args  []any
	}{
		{model: &tenantEntitlementRow{}, query: "module_code = ?", args: []any{moduleCode}},
		{model: &subscriptionPlanModuleRow{}, query: "module_code = ?", args: []any{moduleCode}},
		{model: &subscriptionModuleRow{}, query: "code = ?", args: []any{moduleCode}},
	}
	for _, deletion := range deletions {
		if err := tx.Where(deletion.query, deletion.args...).Delete(deletion.model).Error; err != nil {
			return err
		}
	}
	return nil
}

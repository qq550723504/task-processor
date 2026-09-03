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
	eventIDs := tx.Model(&usageEventRow{}).Select("event_id").Where("module_code = ?", moduleCode)
	deletions := []struct {
		model any
		query string
		args  []any
	}{
		{model: &usageEventOutboxRow{}, query: "event_id IN (?)", args: []any{eventIDs}},
		{model: &usageEventRow{}, query: "module_code = ?", args: []any{moduleCode}},
		{model: &usageBucketRow{}, query: "module_code = ?", args: []any{moduleCode}},
		{model: &usageCounterAdjustmentRow{}, query: "module_code = ?", args: []any{moduleCode}},
		{model: &usageCounterRow{}, query: "module_code = ?", args: []any{moduleCode}},
		{model: &tenantEntitlementRow{}, query: "module_code = ?", args: []any{moduleCode}},
		{model: &subscriptionPlanModuleRow{}, query: "module_code = ?", args: []any{moduleCode}},
		{model: &auditLogRow{}, query: "module_code = ?", args: []any{moduleCode}},
		{model: &subscriptionModuleRow{}, query: "code = ?", args: []any{moduleCode}},
	}
	for _, deletion := range deletions {
		if err := tx.Where(deletion.query, deletion.args...).Delete(deletion.model).Error; err != nil {
			return err
		}
	}
	return nil
}

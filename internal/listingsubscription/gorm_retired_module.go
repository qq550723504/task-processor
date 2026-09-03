package listingsubscription

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

const retiredSystemStudioModuleCode = "studio"

func (r *GormRepository) SyncDefaultCatalog(ctx context.Context, modules []Module, plans []PlanBundle) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := migrateRetiredStudioEntitlements(tx, plans); err != nil {
			return err
		}
		if err := retireSystemModule(tx, retiredSystemStudioModuleCode); err != nil {
			return err
		}
		if err := upsertDefaultModules(tx, modules); err != nil {
			return err
		}
		return upsertDefaultPlans(tx, plans)
	})
}

// migrateRetiredStudioEntitlements converts active Studio entitlements into
// ListingKit entitlements inside the same transaction that retires the module.
// Tenants must not lose ListingKit access merely because the startup catalog
// sync removed the module row before an administrator reapplies a plan.
func migrateRetiredStudioEntitlements(tx *gorm.DB, plans []PlanBundle) error {
	var retired []tenantEntitlementRow
	if err := tx.Where("module_code = ? AND status IN ?", retiredSystemStudioModuleCode, []string{StatusActive, StatusTrialing}).
		Find(&retired).Error; err != nil {
		return err
	}
	if len(retired) == 0 {
		return nil
	}
	planLimits := listingKitPlanLimitsByPlan(plans)
	for _, row := range retired {
		var existing tenantEntitlementRow
		err := tx.Where("tenant_id = ? AND module_code = ?", row.TenantID, ModuleListingKit).Take(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		limits, ok := tenantPlanListingKitLimits(tx, row.TenantID, planLimits)
		if !ok {
			limits = migratedStudioLimits(row.LimitsJSON)
		}
		limitsJSON, err := marshalLimits(limits)
		if err != nil {
			return err
		}
		replacement := tenantEntitlementRow{
			TenantID: row.TenantID, ModuleCode: ModuleListingKit, Status: row.Status,
			StartsAt: row.StartsAt, ExpiresAt: row.ExpiresAt, LimitsJSON: limitsJSON,
		}
		if err := tx.Create(&replacement).Error; err != nil {
			return err
		}
	}
	return nil
}

func listingKitPlanLimitsByPlan(plans []PlanBundle) map[string]map[string]int {
	planLimits := make(map[string]map[string]int)
	for _, plan := range plans {
		for _, module := range plan.Modules {
			if module.ModuleCode == ModuleListingKit {
				planLimits[plan.Plan.Code] = module.Limits
			}
		}
	}
	return planLimits
}

func tenantPlanListingKitLimits(tx *gorm.DB, tenantID string, planLimits map[string]map[string]int) (map[string]int, bool) {
	var subscription tenantSubscriptionRow
	if err := tx.Where("tenant_id = ?", tenantID).Take(&subscription).Error; err != nil {
		return nil, false
	}
	limits, ok := planLimits[subscription.PlanCode]
	if !ok {
		return nil, false
	}
	return cloneLimits(limits), true
}

func migratedStudioLimits(limitsJSON string) map[string]int {
	limits, err := unmarshalLimits(limitsJSON)
	if err != nil || len(limits) == 0 {
		return nil
	}
	migrated := make(map[string]int, len(limits)+1)
	for key, value := range limits {
		if key == "design_jobs" {
			migrated["listingkit_generations_succeeded"] = value
			continue
		}
		migrated[key] = value
	}
	return migrated
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

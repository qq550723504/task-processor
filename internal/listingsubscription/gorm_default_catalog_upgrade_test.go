package listingsubscription

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
)

const retiredStudioModuleCode = "studio"

func TestNewServiceRetiresStudioCatalogAndPreservesLedgerRowsAtomically(t *testing.T) {
	db := openUsageLedgerTestDB(t)
	seedRetiredStudioSubscriptionGraph(t, db)

	service, err := NewService(NewGormRepository(db))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	modules, err := service.ListModules(context.Background())
	if err != nil {
		t.Fatalf("ListModules() error = %v", err)
	}
	assertModuleCodes(t, modules, ModuleListingKit, "custom_studio_extension")
	for _, module := range modules {
		if module.Code == retiredStudioModuleCode {
			t.Fatalf("ListModules() retained retired module: %+v", module)
		}
	}

	plans, err := service.ListPlans(context.Background())
	if err != nil {
		t.Fatalf("ListPlans() error = %v", err)
	}
	professional := findPlanBundle(t, plans, PlanProfessional)
	assertPlanModuleCodes(t, professional.Modules, ModuleListingKit)
	for _, module := range professional.Modules {
		if module.ModuleCode == retiredStudioModuleCode {
			t.Fatalf("professional plan retained retired module: %+v", module)
		}
	}
	custom := findPlanBundle(t, plans, "custom_plan")
	assertPlanModuleCodes(t, custom.Modules, "custom_studio_extension")

	summary, err := service.GetTenantSummary(context.Background(), "tenant-upgrade")
	if err != nil {
		t.Fatalf("GetTenantSummary() error = %v", err)
	}
	var migratedListingKit *EntitlementView
	for _, view := range summary.Entitlements {
		if view.Module.Code == retiredStudioModuleCode {
			t.Fatalf("tenant summary retained retired entitlement: %+v", view)
		}
		if view.Module.Code == ModuleListingKit {
			migratedListingKit = &view
		}
	}
	if migratedListingKit == nil || !migratedListingKit.Allowed {
		t.Fatalf("listingkit entitlement = %+v, want migrated active entitlement", migratedListingKit)
	}
	if migratedListingKit.Entitlement == nil || migratedListingKit.Entitlement.Status != StatusActive {
		t.Fatalf("migrated listingkit entitlement status = %+v, want active", migratedListingKit.Entitlement)
	}
	if migratedListingKit.Entitlement.Limits["listingkit_generations_succeeded"] != 100 || migratedListingKit.Entitlement.Limits["product_image_jobs"] != 100 {
		t.Fatalf("migrated listingkit limits = %+v, want professional default plan limits", migratedListingKit.Entitlement.Limits)
	}

	assertRetiredStudioCatalogRetiredAndLedgerPresent(t, db)
}

func TestNewServiceRollsBackStudioRetirementWhenDefaultCatalogSyncFails(t *testing.T) {
	db := openUsageLedgerTestDB(t)
	seedRetiredStudioSubscriptionGraph(t, db)
	if err := db.Exec(`CREATE TRIGGER fail_listingkit_default BEFORE INSERT ON saas_modules
		WHEN NEW.code = 'listingkit' BEGIN SELECT RAISE(ABORT, 'forced default sync failure'); END`).Error; err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	if _, err := NewService(NewGormRepository(db)); err == nil {
		t.Fatal("NewService() error = nil, want forced transaction failure")
	}

	assertRetiredStudioGraphPresent(t, db)
	var insertedDefaults int64
	if err := db.Model(&subscriptionModuleRow{}).
		Where("code IN ?", []string{ModuleStoreManagement, ModuleTaskImport, ModuleRules, ModuleOperationStrategy, ModuleListingKit, ModuleOSSStorage}).
		Count(&insertedDefaults).Error; err != nil {
		t.Fatalf("count partially inserted defaults: %v", err)
	}
	if insertedDefaults != 0 {
		t.Fatalf("partially inserted default modules = %d, want 0 after rollback", insertedDefaults)
	}
}

func TestNewServiceMigratesStudioEntitlementsWithoutPlanOrExistingReplacement(t *testing.T) {
	db := openUsageLedgerTestDB(t)
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	seed := []any{
		&subscriptionModuleRow{Code: retiredStudioModuleCode, Name: "Studio", Active: true},
		&tenantEntitlementRow{TenantID: "tenant-no-plan", ModuleCode: retiredStudioModuleCode, Status: StatusActive, LimitsJSON: `{"design_jobs":7,"product_image_jobs":9,"shein_drafts_succeeded":11}`, ExpiresAt: &now},
		&tenantEntitlementRow{TenantID: "tenant-has-listingkit", ModuleCode: retiredStudioModuleCode, Status: StatusActive, LimitsJSON: `{"design_jobs":7}`},
		&tenantEntitlementRow{TenantID: "tenant-has-listingkit", ModuleCode: ModuleListingKit, Status: StatusActive, LimitsJSON: `{"listingkit_generations_succeeded":42}`},
		&tenantEntitlementRow{TenantID: "tenant-expired", ModuleCode: retiredStudioModuleCode, Status: StatusExpired, LimitsJSON: `{"design_jobs":5}`},
	}
	for _, row := range seed {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed %T: %v", row, err)
		}
	}

	if _, err := NewService(NewGormRepository(db)); err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	var noPlan tenantEntitlementRow
	if err := db.Where("tenant_id = ? AND module_code = ?", "tenant-no-plan", ModuleListingKit).Take(&noPlan).Error; err != nil {
		t.Fatalf("migrated no-plan entitlement missing: %v", err)
	}
	if noPlan.Status != StatusActive || noPlan.ExpiresAt == nil || !noPlan.ExpiresAt.Equal(now) {
		t.Fatalf("migrated no-plan entitlement = %+v, want active with preserved expiry", noPlan)
	}
	noPlanLimits, err := unmarshalLimits(noPlan.LimitsJSON)
	if err != nil {
		t.Fatalf("unmarshal migrated limits: %v", err)
	}
	if noPlanLimits["listingkit_generations_succeeded"] != 7 || noPlanLimits["product_image_jobs"] != 9 || noPlanLimits["shein_drafts_succeeded"] != 11 {
		t.Fatalf("migrated no-plan limits = %+v, want mapped studio metrics", noPlanLimits)
	}

	var existing tenantEntitlementRow
	if err := db.Where("tenant_id = ? AND module_code = ?", "tenant-has-listingkit", ModuleListingKit).Take(&existing).Error; err != nil {
		t.Fatalf("existing listingkit entitlement missing: %v", err)
	}
	existingLimits, err := unmarshalLimits(existing.LimitsJSON)
	if err != nil {
		t.Fatalf("unmarshal existing limits: %v", err)
	}
	if existingLimits["listingkit_generations_succeeded"] != 42 {
		t.Fatalf("existing listingkit limits = %+v, want untouched value 42", existingLimits)
	}

	assertRowCount(t, db, &tenantEntitlementRow{}, "tenant_id = ? AND module_code = ?", 0, "tenant-expired", ModuleListingKit)
}

func seedRetiredStudioSubscriptionGraph(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	rows := []any{
		&subscriptionModuleRow{Code: retiredStudioModuleCode, Name: "Studio", Active: true},
		&subscriptionModuleRow{Code: "custom_studio_extension", Name: "Custom", Active: true},
		&subscriptionPlanRow{Code: PlanProfessional, Name: "Professional", Active: true},
		&subscriptionPlanRow{Code: "custom_plan", Name: "Custom plan", Active: true},
		&subscriptionPlanModuleRow{PlanCode: PlanProfessional, ModuleCode: retiredStudioModuleCode, LimitsJSON: `{}`},
		&subscriptionPlanModuleRow{PlanCode: "custom_plan", ModuleCode: "custom_studio_extension", LimitsJSON: `{}`},
		&tenantSubscriptionRow{TenantID: "tenant-upgrade", PlanCode: PlanProfessional, Status: StatusActive},
		&tenantEntitlementRow{TenantID: "tenant-upgrade", ModuleCode: retiredStudioModuleCode, Status: StatusActive, LimitsJSON: `{}`},
		&usageCounterRow{TenantID: "tenant-upgrade", ModuleCode: retiredStudioModuleCode, PeriodKey: "2026-08", Metric: "studio_metric", Used: 1},
		&usageCounterAdjustmentRow{OperationKey: "studio-adjustment", TenantID: "tenant-upgrade", ModuleCode: retiredStudioModuleCode, PeriodKey: "2026-08", Metric: "studio_metric", Amount: 1},
		&usageEventRow{EventID: "studio-event", TenantID: "tenant-upgrade", ModuleCode: retiredStudioModuleCode, Metric: "studio_metric", Quantity: 1, PeriodKey: "2026-08", SourceType: "studio", SourceID: "studio-source", IdempotencyKey: "studio-event", Status: string(UsageEventCommitted), OccurredAt: now},
		&usageEventOutboxRow{EventID: "studio-event", Destination: "openmeter", Status: "pending"},
		&usageBucketRow{TenantID: "tenant-upgrade", ModuleCode: retiredStudioModuleCode, PeriodKey: "2026-08", Metric: "studio_metric", Committed: 1},
		&auditLogRow{TenantID: "tenant-upgrade", ModuleCode: retiredStudioModuleCode, Action: "studio_action"},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("seed %T: %v", row, err)
		}
	}
}

func assertRetiredStudioCatalogRetiredAndLedgerPresent(t *testing.T, db *gorm.DB) {
	t.Helper()
	assertRowCount(t, db, &subscriptionModuleRow{}, "code = ?", 0, retiredStudioModuleCode)
	assertRowCount(t, db, &subscriptionPlanModuleRow{}, "module_code = ?", 0, retiredStudioModuleCode)
	assertRowCount(t, db, &tenantEntitlementRow{}, "module_code = ?", 0, retiredStudioModuleCode)
	assertRowCount(t, db, &usageCounterRow{}, "module_code = ?", 1, retiredStudioModuleCode)
	assertRowCount(t, db, &usageCounterAdjustmentRow{}, "module_code = ?", 1, retiredStudioModuleCode)
	assertRowCount(t, db, &usageEventRow{}, "module_code = ?", 1, retiredStudioModuleCode)
	assertRowCount(t, db, &usageBucketRow{}, "module_code = ?", 1, retiredStudioModuleCode)
	assertRowCount(t, db, &auditLogRow{}, "module_code = ?", 1, retiredStudioModuleCode)
	assertRowCount(t, db, &usageEventOutboxRow{}, "event_id = ?", 1, "studio-event")
}

func assertRetiredStudioGraphPresent(t *testing.T, db *gorm.DB) {
	t.Helper()
	assertRowCount(t, db, &subscriptionModuleRow{}, "code = ?", 1, retiredStudioModuleCode)
	assertRowCount(t, db, &subscriptionPlanModuleRow{}, "module_code = ?", 1, retiredStudioModuleCode)
	assertRowCount(t, db, &tenantEntitlementRow{}, "module_code = ?", 1, retiredStudioModuleCode)
	assertRowCount(t, db, &usageCounterRow{}, "module_code = ?", 1, retiredStudioModuleCode)
	assertRowCount(t, db, &usageCounterAdjustmentRow{}, "module_code = ?", 1, retiredStudioModuleCode)
	assertRowCount(t, db, &usageEventRow{}, "module_code = ?", 1, retiredStudioModuleCode)
	assertRowCount(t, db, &usageBucketRow{}, "module_code = ?", 1, retiredStudioModuleCode)
	assertRowCount(t, db, &auditLogRow{}, "module_code = ?", 1, retiredStudioModuleCode)
	assertRowCount(t, db, &usageEventOutboxRow{}, "event_id = ?", 1, "studio-event")
}

func assertRowCount(t *testing.T, db *gorm.DB, model any, query string, want int64, args ...any) {
	t.Helper()
	var got int64
	if err := db.Model(model).Where(query, args...).Count(&got).Error; err != nil {
		t.Fatalf("count %T: %v", model, err)
	}
	if got != want {
		t.Fatalf("count %T where %q = %d, want %d", model, query, got, want)
	}
}

func assertModuleCodes(t *testing.T, modules []Module, wanted ...string) {
	t.Helper()
	for _, code := range wanted {
		found := false
		for _, module := range modules {
			found = found || module.Code == code
		}
		if !found {
			t.Fatalf("modules = %+v, missing %q", modules, code)
		}
	}
}

func findPlanBundle(t *testing.T, plans []PlanBundle, code string) PlanBundle {
	t.Helper()
	for _, plan := range plans {
		if plan.Plan.Code == code {
			return plan
		}
	}
	t.Fatalf("plans = %+v, missing %q", plans, code)
	return PlanBundle{}
}

func assertPlanModuleCodes(t *testing.T, modules []PlanModule, wanted ...string) {
	t.Helper()
	for _, code := range wanted {
		found := false
		for _, module := range modules {
			found = found || module.ModuleCode == code
		}
		if !found {
			t.Fatalf("plan modules = %+v, missing %q", modules, code)
		}
	}
}

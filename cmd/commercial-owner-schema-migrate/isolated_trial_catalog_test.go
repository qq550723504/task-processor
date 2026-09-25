package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	"task-processor/internal/listingsubscription"
)

func trialCatalogDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, listingsubscription.AutoMigrateRepository(db))
	require.NoError(t, commercialstore.AutoMigrate(db))
	return db
}

func TestProvisionIsolatedTrialCatalogCreatesOnlyChosenPlanAndOffer(t *testing.T) {
	db := trialCatalogDB(t)
	now := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	require.NoError(t, provisionIsolatedTrialCatalog(context.Background(), db, now))

	owner, err := listingsubscription.NewRuntimeService(listingsubscription.NewGormRepository(db))
	require.NoError(t, err)
	snapshot, err := owner.ResolvePurchasablePlan(context.Background(), "paid_pilot")
	require.NoError(t, err)
	require.NotEmpty(t, snapshot.Fingerprint)
	require.Equal(t, "paid_pilot", snapshot.PlanCode)

	var count int64
	require.NoError(t, db.Table("saas_plans").Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Table("commercial_offers").Count(&count).Error)
	require.EqualValues(t, 1, count)

	commercial, err := commercialstore.New(db)
	require.NoError(t, err)
	offer, err := commercial.ReadOffer(context.Background(), "paid-pilot-isolated-trial-v1")
	require.NoError(t, err)
	require.Equal(t, "paid_pilot", offer.PlanCode)
	require.EqualValues(t, 0, offer.UnitPriceMinor)
	require.Equal(t, now.Add(48*time.Hour), *offer.ExpiresAt)

	for module, expected := range map[string]string{
		listingsubscription.ModuleStoreManagement: `{"store_count":1}`,
		listingsubscription.ModuleRules:           `{}`,
		listingsubscription.ModuleListingKit:      `{"listingkit_generations_succeeded":5,"product_image_jobs_succeeded":5,"shein_drafts_succeeded":5,"ai_tokens":50000}`,
		listingsubscription.ModuleOSSStorage:      `{"storage_bytes_current":104857600,"storage_bytes":104857600}`,
	} {
		var limits string
		require.NoError(t, db.Table("saas_plan_modules").Select("limits").Where("plan_code = ? AND module_code = ?", "paid_pilot", module).Scan(&limits).Error)
		require.JSONEq(t, expected, limits, module)
	}
	require.NoError(t, db.Table("saas_plan_modules").Count(&count).Error)
	require.EqualValues(t, 4, count)
	require.NoError(t, db.Table("saas_plan_modules").Where("module_code = ?", "shein_publish").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Table("saas_tenant_entitlements").Count(&count).Error)
	require.Zero(t, count)
}

func TestProvisionIsolatedTrialCatalogRefusesExistingDataWithoutMutation(t *testing.T) {
	db := trialCatalogDB(t)
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	require.NoError(t, provisionIsolatedTrialCatalog(context.Background(), db, now))
	require.Error(t, provisionIsolatedTrialCatalog(context.Background(), db, now.Add(time.Hour)))
	var count int64
	require.NoError(t, db.Table("commercial_offers").Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, db.Table("saas_plans").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestProvisionIsolatedTrialCatalogRejectsInvalidTimeWithoutWrites(t *testing.T) {
	db := trialCatalogDB(t)
	require.Error(t, provisionIsolatedTrialCatalog(context.Background(), db, time.Time{}))
	var count int64
	require.NoError(t, db.Table("commercial_offers").Count(&count).Error)
	require.Zero(t, count)
}

func TestProvisionIsolatedTrialCatalogRollsBackPlanWhenOfferInsertFails(t *testing.T) {
	db := trialCatalogDB(t)
	require.NoError(t, db.Exec(`CREATE TRIGGER fail_trial_offer BEFORE INSERT ON commercial_offers BEGIN SELECT RAISE(FAIL, 'offer rejected'); END`).Error)
	require.Error(t, provisionIsolatedTrialCatalog(context.Background(), db, time.Now().UTC()))
	for _, table := range []string{"saas_modules", "saas_plans", "saas_plan_modules", "commercial_offers"} {
		var count int64
		require.NoError(t, db.Table(table).Count(&count).Error)
		require.Zero(t, count, table)
	}
}

func TestRunIsolatedTrialRequiresAbsolutePrivatePath(t *testing.T) {
	err := runIsolatedTrial("relative-private-db.json", "subscription_trial")
	require.ErrorContains(t, err, "absolute")
}

func TestRunRejectsProductionNamedDatabaseBeforeConnecting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private-db.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"host":"127.0.0.1","port":5432,"user":"schema_owner","password":"test-only","database":"production","maxConnections":1,"maxIdleConnections":0}`), 0600))
	require.ErrorContains(t, runIsolatedTrial(path, "production"), "isolated or trial")
}

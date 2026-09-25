package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"task-processor/internal/commercial/billing"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	"task-processor/internal/listingsubscription"
	platformdatabase "task-processor/internal/platform/database"
)

const (
	trialPlanCode = "paid_pilot"
	trialOfferID  = "paid-pilot-isolated-trial-v1"
)

// runIsolatedTrial is an explicit offline mode of the existing schema-owner
// command. It never runs as part of application startup or normal migration.
func runIsolatedTrial(configPath, expectedDatabase string) error {
	if !filepath.IsAbs(configPath) {
		return errors.New("private database config path must be absolute")
	}
	config, err := loadSchemaOwnerConfig(configPath)
	if err != nil {
		return err
	}
	if config.Database != expectedDatabase || strings.TrimSpace(expectedDatabase) == "" {
		return errors.New("database name does not match explicit isolated target")
	}
	lowerDatabase := strings.ToLower(config.Database)
	if !strings.Contains(lowerDatabase, "isolated") && !strings.Contains(lowerDatabase, "trial") {
		return errors.New("isolated trial database name must contain isolated or trial")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := platformdatabase.OpenExistingWritableContext(ctx, config)
	if err != nil {
		return fmt.Errorf("open existing commercial owner database: %w", err)
	}
	defer platformdatabase.Close(db)
	var schemaOwner bool
	if err := db.WithContext(ctx).Raw("SELECT current_schema() = 'public' AND has_schema_privilege(current_user, 'public', 'CREATE')").Scan(&schemaOwner).Error; err != nil {
		return fmt.Errorf("verify commercial schema-owner privileges: %w", err)
	}
	if !schemaOwner {
		return errors.New("isolated catalog provisioning requires the commercial schema-owner role")
	}
	return provisionIsolatedTrialCatalog(ctx, db, time.Now().UTC())
}

func provisionIsolatedTrialCatalog(ctx context.Context, db *gorm.DB, now time.Time) error {
	if db == nil || now.IsZero() {
		return errors.New("database and current time are required")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// A trial catalog must never overwrite a deployed catalog, offer, quote,
		// order, subscription or entitlement. This command is for an empty
		// isolated database after schema migration and before serving traffic.
		for _, table := range []string{
			"saas_modules", "saas_plans", "saas_plan_modules",
			"saas_tenant_subscriptions", "saas_tenant_entitlements", "saas_purchased_plan_activations",
			"saas_subscription_activation_fences", "saas_subscription_audit_logs",
			"commercial_offers", "commercial_quotes", "commercial_orders", "commercial_order_items",
		} {
			var count int64
			if err := tx.Table(table).Count(&count).Error; err != nil {
				return fmt.Errorf("check migrated catalog table %s: %w", table, err)
			}
			if count != 0 {
				return fmt.Errorf("isolated catalog provisioning requires an empty %s table", table)
			}
		}
		planRepo := listingsubscription.NewGormRepository(tx)
		if err := planRepo.CreateCatalogPlan(ctx, trialModules(), trialPlan()); err != nil {
			return fmt.Errorf("create trial plan: %w", err)
		}
		commercialRepo, err := commercialstore.New(tx)
		if err != nil {
			return err
		}
		if err := commercialRepo.CreateOffer(ctx, trialOffer(now.UTC())); err != nil {
			return fmt.Errorf("create trial offer: %w", err)
		}
		owner, err := listingsubscription.NewRuntimeService(planRepo)
		if err != nil {
			return err
		}
		if _, err := owner.ResolvePurchasablePlan(ctx, trialPlanCode); err != nil {
			return fmt.Errorf("verify purchased plan snapshot: %w", err)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
}

func trialModules() []listingsubscription.Module {
	return []listingsubscription.Module{
		{Code: listingsubscription.ModuleStoreManagement, Name: "店铺管理", Active: true, SortOrder: 10},
		{Code: listingsubscription.ModuleRules, Name: "规则", Active: true, SortOrder: 20},
		{Code: listingsubscription.ModuleListingKit, Name: "ListingKit", Active: true, SortOrder: 30},
		{Code: listingsubscription.ModuleOSSStorage, Name: "OSS 存储", Active: true, SortOrder: 40},
	}
}

func trialPlan() listingsubscription.PlanBundle {
	return listingsubscription.PlanBundle{
		Plan: listingsubscription.Plan{Code: trialPlanCode, Name: "付费试点（隔离试用）", Description: "仅供隔离环境验证自助开通，不代表生产套餐或价格", Active: true, SortOrder: 10},
		Modules: []listingsubscription.PlanModule{
			{PlanCode: trialPlanCode, ModuleCode: listingsubscription.ModuleStoreManagement, Limits: map[string]int{"store_count": 1}, SortOrder: 10},
			{PlanCode: trialPlanCode, ModuleCode: listingsubscription.ModuleRules, SortOrder: 20},
			{PlanCode: trialPlanCode, ModuleCode: listingsubscription.ModuleListingKit, Limits: map[string]int{"listingkit_generations_succeeded": 5, "product_image_jobs_succeeded": 5, "shein_drafts_succeeded": 5, "ai_tokens": 50000}, SortOrder: 30},
			{PlanCode: trialPlanCode, ModuleCode: listingsubscription.ModuleOSSStorage, Limits: map[string]int{"storage_bytes_current": 100 * 1024 * 1024, "storage_bytes": 100 * 1024 * 1024}, SortOrder: 40},
		},
	}
}

func trialOffer(now time.Time) billing.Offer {
	starts, expires := now, now.Add(48*time.Hour)
	return billing.Offer{
		OfferID: trialOfferID, ProductKind: billing.ProductSubscriptionPlan,
		PlanCode: trialPlanCode, TermMonths: 1, SettlementMode: billing.SettlementZeroPrice,
		Currency: billing.CurrencyCNY, UnitPriceMinor: 0, PricingVersion: "isolated-trial-v1",
		Status: billing.OfferActive, StartsAt: &starts, ExpiresAt: &expires,
	}
}

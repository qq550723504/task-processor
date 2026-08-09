package schema

import (
	"fmt"

	"gorm.io/gorm"

	aicapabilitystore "task-processor/internal/aicapability/store"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/memberinvite"
	"task-processor/internal/listingkit/reviewstore"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/listingsubscription"
	sheinpub "task-processor/internal/publishing/shein"
)

// AutoMigrateRuntime creates and updates every table required by ListingKit.
func AutoMigrateRuntime(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if err := autoMigrateTaskRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit task repository: %w", err)
	}
	if err := memberinvite.AutoMigrateAuditRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit member invitation audit repository: %w", err)
	}
	if err := aicapabilitystore.AutoMigrateInvocationLedger(db); err != nil {
		return fmt.Errorf("ai invocation ledger auto-migrate failed: %w", err)
	}
	if err := aicapabilitystore.AutoMigrateAsyncJobBindings(db); err != nil {
		return fmt.Errorf("ai async job binding auto-migrate failed: %w", err)
	}
	if err := listingkit.AutoMigrateStudioAsyncJobRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit studio async job repository: %w", err)
	}
	if err := listingkit.AutoMigrateStudioBatchRunRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit studio batch run repository: %w", err)
	}
	if err := listingkit.AutoMigrateStudioBatchRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit studio batch repository: %w", err)
	}
	if err := listingkit.AutoMigrateStudioBatchTaskLinkRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit studio batch task link repository: %w", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSChildRetryJob{}); err != nil {
		return fmt.Errorf("migrate listingkit sds child retry repository: %w", err)
	}
	if err := listingkitstore.AutoMigrateSheinSyncRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit shein sync repository: %w", err)
	}
	if err := db.AutoMigrate(&listingkit.SDSRetirementRunRecord{}, &listingkit.SDSRetirementItemRecord{}); err != nil {
		return fmt.Errorf("migrate listingkit sds retirement repository: %w", err)
	}
	if err := listingkit.AutoMigrateUploadedImageRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit uploaded image repository: %w", err)
	}
	if err := listingkit.AutoMigrateStoreProfileRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit store profile repository: %w", err)
	}
	if err := listingadmin.AutoMigrateStoreRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin store repository: %w", err)
	}
	if err := listingadmin.AutoMigrateStoreStatisticsRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin store statistics repository: %w", err)
	}
	if err := listingadmin.AutoMigrateImportTaskRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin import task repository: %w", err)
	}
	if err := listingadmin.AutoMigrateFilterRuleRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin filter rule repository: %w", err)
	}
	if err := listingadmin.AutoMigrateProfitRuleRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin profit rule repository: %w", err)
	}
	if err := listingadmin.AutoMigratePricingRuleRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin pricing rule repository: %w", err)
	}
	if err := listingadmin.AutoMigrateOperationStrategyRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin operation strategy repository: %w", err)
	}
	if err := listingadmin.AutoMigrateScheduledTaskConfigRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin scheduled task config repository: %w", err)
	}
	if err := listingadmin.AutoMigrateSensitiveWordRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin sensitive word repository: %w", err)
	}
	if err := listingadmin.AutoMigrateGenerationTopicPolicyRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin generation topic policy repository: %w", err)
	}
	if err := listingadmin.AutoMigrateGenerationTopicOverrideRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin generation topic override repository: %w", err)
	}
	if err := listingadmin.AutoMigrateProductImportMappingRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin product import mapping repository: %w", err)
	}
	if err := listingadmin.AutoMigrateCategoryRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin category repository: %w", err)
	}
	if err := listingadmin.AutoMigrateProductDataRepository(db); err != nil {
		return fmt.Errorf("migrate listingadmin product data repository: %w", err)
	}
	if err := db.AutoMigrate(&sheinpub.SheinResolutionCacheEntry{}); err != nil {
		return fmt.Errorf("migrate shein resolution cache store: %w", err)
	}
	if err := db.AutoMigrate(&assetrepo.InventorySnapshot{}, &assetrepo.GenerationTaskSnapshot{}); err != nil {
		return fmt.Errorf("migrate asset repository: %w", err)
	}
	if err := db.AutoMigrate(&reviewstore.ReviewRecord{}); err != nil {
		return fmt.Errorf("migrate listingkit review repository: %w", err)
	}
	if err := db.AutoMigrate(&listingkit.SheinStudioSession{}, &listingkit.SheinStudioDesign{}); err != nil {
		return fmt.Errorf("migrate listingkit studio session repository: %w", err)
	}
	if err := listingsubscription.AutoMigrateRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit subscription repository: %w", err)
	}
	return nil
}

func autoMigrateTaskRepository(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&listingkit.Task{},
		&listingkit.CanonicalProductCacheEntry{},
		&listingkit.SDSBaselineCacheEntry{},
	); err != nil {
		return err
	}
	return listingkitstore.AutoMigrateSheinPODImageLookupIndex(db)
}

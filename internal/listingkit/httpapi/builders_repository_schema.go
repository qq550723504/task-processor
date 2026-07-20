package httpapi

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"gorm.io/gorm"

	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/core/config"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/reviewstore"
	listingkitstore "task-processor/internal/listingkit/store"
	"task-processor/internal/listingsubscription"
	sheinpub "task-processor/internal/publishing/shein"
)

type repositorySchemaBootstrapper struct {
	mu      sync.Mutex
	entries map[string]*repositorySchemaBootstrapEntry
}

type repositorySchemaBootstrapEntry struct {
	once sync.Once
	err  error
}

func newRepositorySchemaBootstrapper() *repositorySchemaBootstrapper {
	return &repositorySchemaBootstrapper{
		entries: make(map[string]*repositorySchemaBootstrapEntry),
	}
}

func (b *repositorySchemaBootstrapper) ensure(cfg *config.DatabaseConfig, run func() error) error {
	if b == nil || run == nil {
		return nil
	}

	key := repositorySchemaKey(cfg)

	b.mu.Lock()
	entry := b.entries[key]
	if entry == nil {
		entry = &repositorySchemaBootstrapEntry{}
		b.entries[key] = entry
	}
	b.mu.Unlock()

	entry.once.Do(func() {
		entry.err = run()
	})
	return entry.err
}

func repositorySchemaKey(cfg *config.DatabaseConfig) string {
	if cfg == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d:%s:%s", cfg.Host, cfg.Port, cfg.User, cfg.Database)
}

// listingKitRepositorySchemaBootstrapper is the default bootstrapper for schema migrations.
// It can be overridden in tests for isolation.
var listingKitRepositorySchemaBootstrapper = newRepositorySchemaBootstrapper()

// SetRepositorySchemaBootstrapper allows tests to override the global bootstrapper.
// This should only be used in test code.
func SetRepositorySchemaBootstrapper(b *repositorySchemaBootstrapper) {
	listingKitRepositorySchemaBootstrapper = b
}

func ensureListingKitRepositorySchema(cfg *config.DatabaseConfig, db *gorm.DB) error {
	if !shouldAutoMigrateListingKitRuntime() {
		return nil
	}
	return listingKitRepositorySchemaBootstrapper.ensure(cfg, func() error {
		return runListingKitRepositoryAutoMigrations(db)
	})
}

func shouldAutoMigrateListingKitRuntime() bool {
	raw := strings.TrimSpace(os.Getenv("TASK_PROCESSOR_LISTINGKIT_RUNTIME_AUTOMIGRATE"))
	if raw == "" {
		return true
	}
	switch strings.ToLower(raw) {
	case "0", "false", "no", "n", "off", "disabled":
		return false
	default:
		return true
	}
}

func AutoMigrateListingKitRuntimeSchema(db *gorm.DB) error {
	return runListingKitRepositoryAutoMigrations(db)
}

func runListingKitRepositoryAutoMigrations(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if err := autoMigrateListingKitTaskRepository(db); err != nil {
		return fmt.Errorf("migrate listingkit task repository: %w", err)
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

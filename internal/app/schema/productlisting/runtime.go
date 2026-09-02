package productlisting

import (
	"fmt"

	"gorm.io/gorm"

	aicapabilitystore "task-processor/internal/aicapability/store"
	"task-processor/internal/amazonlisting"
	imageagentstore "task-processor/internal/imageagent/store"
	openaiclient "task-processor/internal/integration/openai"
	assetpersistence "task-processor/internal/integration/persistence/product/asset"
	catalogpersistence "task-processor/internal/integration/persistence/product/catalog"
	"task-processor/internal/productenrich"
	productimage "task-processor/internal/productimage"
	"task-processor/internal/prompt"
)

// AutoMigrateRuntime creates the schema required by the product-listing API,
// including ProductImage task identity columns used by governed AI calls.
func AutoMigrateRuntime(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if err := db.AutoMigrate(&openaiclient.AIClientCredential{}); err != nil {
		return fmt.Errorf("openai credential auto-migrate failed: %w", err)
	}
	if err := aicapabilitystore.AutoMigrateInvocationLedger(db); err != nil {
		return fmt.Errorf("ai invocation ledger auto-migrate failed: %w", err)
	}
	if err := aicapabilitystore.AutoMigrateAsyncJobBindings(db); err != nil {
		return fmt.Errorf("ai async job binding auto-migrate failed: %w", err)
	}
	if err := imageagentstore.AutoMigrate(db); err != nil {
		return fmt.Errorf("image agent auto-migrate failed: %w", err)
	}
	if err := assetpersistence.AutoMigrate(db); err != nil {
		return fmt.Errorf("approved product asset auto-migrate failed: %w", err)
	}
	if err := catalogpersistence.AutoMigrate(db); err != nil {
		return fmt.Errorf("product snapshot auto-migrate failed: %w", err)
	}
	if err := db.AutoMigrate(&prompt.TenantPromptTemplate{}); err != nil {
		return fmt.Errorf("tenant prompt auto-migrate failed: %w", err)
	}
	if err := db.AutoMigrate(&productenrich.Task{}); err != nil {
		return fmt.Errorf("productenrich auto-migrate failed: %w", err)
	}
	if err := db.AutoMigrate(&productimage.Task{}); err != nil {
		return fmt.Errorf("productimage auto-migrate failed: %w", err)
	}
	if err := db.AutoMigrate(&amazonlisting.Task{}); err != nil {
		return fmt.Errorf("amazonlisting auto-migrate failed: %w", err)
	}
	return nil
}

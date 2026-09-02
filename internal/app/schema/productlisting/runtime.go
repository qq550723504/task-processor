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
	"task-processor/internal/prompt"
)

// AutoMigrateRuntime creates the schema required by the product-listing API.
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
	if err := db.AutoMigrate(&amazonlisting.Task{}); err != nil {
		return fmt.Errorf("amazonlisting auto-migrate failed: %w", err)
	}
	return nil
}

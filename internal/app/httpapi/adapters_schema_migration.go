package httpapi

import (
	"fmt"

	"gorm.io/gorm"

	aicapabilitystore "task-processor/internal/aicapability/store"
	"task-processor/internal/amazonlisting"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
	productimage "task-processor/internal/productimage"
	"task-processor/internal/prompt"
)

func shouldAutoMigrateProductListingAPIRuntime() bool {
	return config.ProductListingAPIRuntimeAutoMigrateEnabled()
}

func AutoMigrateProductListingAPIRuntimeSchema(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if err := db.AutoMigrate(&openaiclient.AIClientCredential{}); err != nil {
		return fmt.Errorf("openai credential auto-migrate failed: %w", err)
	}
	if err := aicapabilitystore.AutoMigrateInvocationLedger(db); err != nil {
		return fmt.Errorf("ai invocation ledger auto-migrate failed: %w", err)
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

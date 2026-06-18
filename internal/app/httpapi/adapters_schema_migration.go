package httpapi

import (
	"fmt"
	"os"
	"strings"

	"gorm.io/gorm"

	"task-processor/internal/amazonlisting"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
	productimage "task-processor/internal/productimage"
	"task-processor/internal/prompt"
)

func shouldAutoMigrateProductListingAPIRuntime() bool {
	raw := strings.TrimSpace(os.Getenv("TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE"))
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

func AutoMigrateProductListingAPIRuntimeSchema(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if err := db.AutoMigrate(&openaiclient.AIClientCredential{}); err != nil {
		return fmt.Errorf("openai credential auto-migrate failed: %w", err)
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

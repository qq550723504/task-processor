package productlistingschemamigrate

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"gorm.io/gorm"

	aicapabilitystore "task-processor/internal/aicapability/store"
	"task-processor/internal/app/configadapter"
	"task-processor/internal/app/runtime/currentapplication"
	"task-processor/internal/core/config"
	imagestore "task-processor/internal/imageagent/store"
	openaiclient "task-processor/internal/integration/openai"
	assetpersistence "task-processor/internal/integration/persistence/product/asset"
	platformdatabase "task-processor/internal/platform/database"
)

// InitializeOrganizationImageAgent is explicit one-shot maintenance for a new,
// empty ImageAgent owner database. It does not run the broad product-listing
// migrator, create roles, or execute during ordinary API/worker startup.
func InitializeOrganizationImageAgent(ctx context.Context, ownerConfigPath, currentManifestPath string) error {
	if ctx == nil || !filepath.IsAbs(ownerConfigPath) || !filepath.IsAbs(currentManifestPath) || ownerConfigPath == currentManifestPath {
		return errors.New("explicit image agent owner config and current application manifest are required")
	}
	owner, err := config.LoadConfigFromFileWithoutValidation(ownerConfigPath)
	if err != nil {
		return fmt.Errorf("load explicit image agent owner config: %w", err)
	}
	current, err := currentapplication.LoadConfig(currentManifestPath)
	if err != nil {
		return fmt.Errorf("load current application manifest: %w", err)
	}
	if err := validateImageAgentGrantTarget(owner, current); err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	db, err := platformdatabase.OpenExistingWritableContext(bounded, configadapter.Database(owner.Database))
	if err != nil {
		return fmt.Errorf("connect explicit image agent owner database: %w", err)
	}
	defer func() { _ = closeDB(db) }()
	return migrateEmptyOrganizationImageAgent(bounded, db)
}

func migrateEmptyOrganizationImageAgent(ctx context.Context, db *gorm.DB) error {
	if ctx == nil || db == nil || db.Dialector.Name() != "postgres" {
		return errors.New("organization image agent init requires PostgreSQL")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(current_database()||':organization-image-agent-init',0))").Error; err != nil {
			return err
		}
		var empty bool
		if err := tx.Raw(`SELECT current_schema()='public' AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema' AND c.relkind IN ('r','p','v','m','f','S'))`).Scan(&empty).Error; err != nil || !empty {
			return errors.New("organization image agent init requires an empty dedicated database")
		}
		if err := tx.AutoMigrate(&openaiclient.AIClientCredential{}); err != nil {
			return fmt.Errorf("migrate image agent credential owner: %w", err)
		}
		if err := aicapabilitystore.AutoMigrateInvocationLedger(tx); err != nil {
			return fmt.Errorf("migrate image agent invocation owner: %w", err)
		}
		if err := imagestore.AutoMigrateOrganizationScope(tx); err != nil {
			return fmt.Errorf("migrate organization image agent owner: %w", err)
		}
		if err := assetpersistence.AutoMigrate(tx); err != nil {
			return fmt.Errorf("migrate approved product asset owner: %w", err)
		}
		return nil
	})
}

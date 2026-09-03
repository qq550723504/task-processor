// Package schema owns the durable tables used by the Workbench runtime.
package schema

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"task-processor/internal/listingsubscription"
	"task-processor/internal/storecenter"
)

// AutoMigrateRuntime migrates only the Store Center and Store quota tables
// owned by the Workbench slice. It intentionally does not invoke broad legacy
// or Subscription repository migrations.
func AutoMigrateRuntime(db *gorm.DB) error {
	if db == nil {
		return errors.New("workbench schema database is required")
	}
	if err := storecenter.AutoMigrateStoreRepository(db); err != nil {
		return fmt.Errorf("migrate Workbench stores: %w", err)
	}
	if err := storecenter.AutoMigrateAuditRepository(db); err != nil {
		return fmt.Errorf("migrate Workbench store audit: %w", err)
	}
	if err := listingsubscription.AutoMigrateStoreQuotaPrerequisites(db); err != nil {
		return fmt.Errorf("migrate Workbench store quota prerequisites: %w", err)
	}
	if err := listingsubscription.AutoMigrateStoreQuotaLedger(db); err != nil {
		return fmt.Errorf("migrate Workbench store quota: %w", err)
	}
	return nil
}

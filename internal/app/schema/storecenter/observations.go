package storecenter

import (
	"context"
	"errors"
	"gorm.io/gorm"
	observationschema "task-processor/internal/integration/persistence/sheinobservations"
)

// InstallObservations is explicit fresh-instance setup, never runtime startup.
// Install rejects an existing observation schema and does not migrate facts.
func InstallObservations(ctx context.Context, records *gorm.DB) error {
	if records == nil {
		return errors.New("Store observation schema-owner unavailable")
	}
	return records.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Table("public.workbench_stores").Count(&count).Error; err != nil || count != 0 {
			return errors.New("observation initialization requires a fresh empty Store instance")
		}
		if err := observationschema.Install(ctx, tx); err != nil {
			return err
		}
		return observationschema.GrantRuntime(ctx, tx, "store_center_runtime")
	})
}

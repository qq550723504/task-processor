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
	var count int64
	if err := records.WithContext(ctx).Table("public.workbench_stores").Count(&count).Error; err != nil || count != 0 {
		return errors.New("observation initialization requires a fresh empty Store instance")
	}
	if err := observationschema.Install(ctx, records); err != nil {
		return err
	}
	return observationschema.GrantRuntime(ctx, records, "store_center_runtime")
}

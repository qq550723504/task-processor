package listingadmin

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestGormScheduledTaskConfigRepositoryUpsertAndListEnabled(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := AutoMigrateScheduledTaskConfigRepository(db); err != nil {
		t.Fatalf("AutoMigrateScheduledTaskConfigRepository() error = %v", err)
	}

	repo := NewGormScheduledTaskConfigRepository(db)
	ctx := context.Background()

	first, err := repo.UpsertScheduledTaskConfig(ctx, &ScheduledTaskConfig{
		TenantID:        246,
		StoreID:         962,
		Platform:        "SHEIN",
		TaskType:        "inventory",
		Enabled:         true,
		IntervalSeconds: 3600,
		Remark:          "initial",
	})
	if err != nil {
		t.Fatalf("UpsertScheduledTaskConfig(first) error = %v", err)
	}
	if first == nil || first.ID <= 0 || !first.Enabled || first.Platform != "shein" || first.TaskType != "inventory" {
		t.Fatalf("first config = %+v", first)
	}

	updated, err := repo.UpsertScheduledTaskConfig(ctx, &ScheduledTaskConfig{
		TenantID:        246,
		StoreID:         962,
		Platform:        "shein",
		TaskType:        "inventory",
		Enabled:         false,
		IntervalSeconds: 7200,
		Remark:          "updated",
	})
	if err != nil {
		t.Fatalf("UpsertScheduledTaskConfig(updated) error = %v", err)
	}
	if updated.ID != first.ID || updated.Enabled || updated.IntervalSeconds != 7200 || updated.Remark != "updated" {
		t.Fatalf("updated config = %+v, first ID=%d", updated, first.ID)
	}

	page, err := repo.ListScheduledTaskConfigs(ctx, ScheduledTaskConfigQuery{
		TenantID: 246,
		StoreID: int64PtrIfPositive(962),
	})
	if err != nil {
		t.Fatalf("ListScheduledTaskConfigs() error = %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v", page)
	}

	if _, err := repo.UpsertScheduledTaskConfig(ctx, &ScheduledTaskConfig{
		TenantID:        246,
		StoreID:         962,
		Platform:        "shein",
		TaskType:        "inventory",
		Enabled:         true,
		IntervalSeconds: 1800,
	}); err != nil {
		t.Fatalf("UpsertScheduledTaskConfig(enabled) error = %v", err)
	}
	if _, err := repo.UpsertScheduledTaskConfig(ctx, &ScheduledTaskConfig{
		TenantID:        246,
		StoreID:         963,
		Platform:        "shein",
		TaskType:        "productSync",
		Enabled:         true,
		IntervalSeconds: 1800,
	}); err != nil {
		t.Fatalf("UpsertScheduledTaskConfig(product sync) error = %v", err)
	}

	enabled, err := repo.ListEnabledScheduledTaskConfigs(ctx, "SHEIN", "inventory")
	if err != nil {
		t.Fatalf("ListEnabledScheduledTaskConfigs() error = %v", err)
	}
	if len(enabled) != 1 || enabled[0].StoreID != 962 || enabled[0].IntervalSeconds != 1800 {
		t.Fatalf("enabled configs = %+v", enabled)
	}
}

package local

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/scheduler"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestLocalRuntimeListsEnabledScheduledTaskConfigsFromResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeScheduledTaskConfigTestDB(t)
	seedLocalRuntimeScheduledTaskConfigs(t, db)

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	configs, err := runtime.ListRuntimeScheduledTaskConfigs(context.Background(), "shein", scheduler.TaskTypeInventory)
	if err != nil {
		t.Fatalf("ListRuntimeScheduledTaskConfigs() error = %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("ListRuntimeScheduledTaskConfigs() = %#v; want one enabled shein inventory config", configs)
	}
	config := configs[0]
	if config.TenantID != 1 || config.StoreID != 101 || config.Platform != "shein" || config.TaskType != "inventory" || !config.Enabled || config.IntervalSeconds != 900 {
		t.Fatalf("ListRuntimeScheduledTaskConfigs() = %#v; want persisted enabled shein inventory config", config)
	}
}

func TestLocalRuntimeListsScheduledTaskConfigStatesFromResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeScheduledTaskConfigTestDB(t)
	seedLocalRuntimeScheduledTaskConfigs(t, db)

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	configs, err := runtime.ListRuntimeScheduledTaskConfigStates(context.Background(), "shein", scheduler.TaskTypeInventory)
	if err != nil {
		t.Fatalf("ListRuntimeScheduledTaskConfigStates() error = %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("ListRuntimeScheduledTaskConfigStates() = %#v; want enabled and disabled shein inventory configs", configs)
	}

	states := make(map[int64]bool, len(configs))
	for _, config := range configs {
		states[config.StoreID] = config.Enabled
	}
	if enabled, ok := states[101]; !ok || !enabled {
		t.Fatalf("scheduled task states = %#v; want store 101 enabled", states)
	}
	if enabled, ok := states[102]; !ok || enabled {
		t.Fatalf("scheduled task states = %#v; want store 102 disabled", states)
	}
}

func newLocalRuntimeScheduledTaskConfigTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := listingadmin.AutoMigrateScheduledTaskConfigRepository(db); err != nil {
		t.Fatalf("migrate scheduled task config: %v", err)
	}
	return db
}

func seedLocalRuntimeScheduledTaskConfigs(t *testing.T, db *gorm.DB) {
	t.Helper()
	repo := listingadmin.NewGormScheduledTaskConfigRepository(db)
	ctx := listingadmin.WithOwnerUserID(context.Background(), "scheduler-owner")
	for _, config := range []listingadmin.ScheduledTaskConfig{
		{TenantID: 1, StoreID: 101, Platform: "shein", TaskType: "inventory", Enabled: true, IntervalSeconds: 900},
		{TenantID: 1, StoreID: 102, Platform: "shein", TaskType: "inventory", Enabled: false, IntervalSeconds: 1800},
		{TenantID: 1, StoreID: 103, Platform: "shein", TaskType: "productSync", Enabled: true, IntervalSeconds: 900},
		{TenantID: 1, StoreID: 104, Platform: "temu", TaskType: "inventory", Enabled: true, IntervalSeconds: 900},
	} {
		if _, err := repo.UpsertScheduledTaskConfig(ctx, &config); err != nil {
			t.Fatalf("seed scheduled task config %#v: %v", config, err)
		}
	}
}

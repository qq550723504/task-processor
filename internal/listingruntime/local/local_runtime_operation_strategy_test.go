package local

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestLocalRuntimeLoadsLatestOperationStrategyFromResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeOperationStrategyTestDB(t)
	if err := db.Table("listing_operation_strategy").Create([]localOperationStrategyRow{
		{ID: 201, TenantID: 1, StoreID: 88, Name: "older strategy", Platform: "shein", Status: 1},
		{ID: 202, TenantID: 1, StoreID: 88, Name: "latest strategy", Platform: "shein", Status: 0, ActivityEnabled: true},
	}).Error; err != nil {
		t.Fatalf("seed operation strategies: %v", err)
	}

	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	strategy, err := runtime.GetRuntimeOperationStrategy(88)
	if err != nil {
		t.Fatalf("GetRuntimeOperationStrategy() error = %v", err)
	}
	if strategy == nil || strategy.ID != 202 || strategy.StoreID != 88 || strategy.Name != "latest strategy" || strategy.Platform != "shein" || strategy.Status != 0 || !strategy.ActivityEnabled {
		t.Fatalf("GetRuntimeOperationStrategy() = %#v; want persisted latest strategy", strategy)
	}
}

func newLocalRuntimeOperationStrategyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&localOperationStrategyRow{}); err != nil {
		t.Fatalf("migrate operation strategy: %v", err)
	}
	return db
}

type localOperationStrategyRow struct {
	ID              int64  `gorm:"column:id;primaryKey"`
	TenantID        int64  `gorm:"column:tenant_id"`
	StoreID         int64  `gorm:"column:store_id"`
	Name            string `gorm:"column:name"`
	Platform        string `gorm:"column:platform"`
	Status          int16  `gorm:"column:status"`
	ActivityEnabled bool   `gorm:"column:activity_enabled"`
	Deleted         int16  `gorm:"column:deleted"`
}

func (localOperationStrategyRow) TableName() string { return "listing_operation_strategy" }

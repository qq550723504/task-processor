package local

import (
	"testing"
	"time"

	"task-processor/internal/listingadmin"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestLocalRuntimeInventoryRecordAPIUsesResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeInventoryRecordTestDB(t)
	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	client := runtime.GetInventoryRecordAPI()
	if client == nil {
		t.Fatal("GetInventoryRecordAPI() returned nil")
	}

	stock1 := 10
	if _, err := client.CreateInventoryRecord(&listingadmin.InventoryRecordCreateReqDTO{
		Platform: "shein", ProductId: "INV-RESOURCE-ONLY", Region: "us", Stock: &stock1, IsAvailable: true, SyncSource: "first",
	}); err != nil {
		t.Fatalf("CreateInventoryRecord(first) error: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	stock2 := 15
	if _, err := client.CreateInventoryRecord(&listingadmin.InventoryRecordCreateReqDTO{
		Platform: "shein", ProductId: "INV-RESOURCE-ONLY", Region: "us", Stock: &stock2, IsAvailable: false, SyncSource: "latest",
	}); err != nil {
		t.Fatalf("CreateInventoryRecord(second) error: %v", err)
	}

	record, err := client.GetLatestInventoryRecord("shein", "INV-RESOURCE-ONLY", "us")
	if err != nil {
		t.Fatalf("GetLatestInventoryRecord() error: %v", err)
	}
	if record == nil || record.Stock == nil || *record.Stock != 15 || record.IsAvailable || record.SyncSource != "latest" {
		t.Fatalf("GetLatestInventoryRecord() = %#v, want latest persisted record", record)
	}
}

func TestLocalDataProviderInventoryRecordCompatibilityPreservesFoundState(t *testing.T) {
	db := newLocalRuntimeInventoryRecordTestDB(t)
	provider := NewLocalDataProviderFromResources(NewRuntimeResources(db, nil))
	t.Cleanup(func() { _ = provider.Close() })

	stock := 15
	id, err := provider.CreateInventoryRecord(&listingadmin.InventoryRecordCreateReqDTO{
		Platform: "shein", ProductId: "INV-PROVIDER-COMPAT", Region: "us", Stock: &stock, IsAvailable: true, SyncSource: "compatibility",
	})
	if err != nil || id == 0 {
		t.Fatalf("CreateInventoryRecord() = %d, %v; want persisted record", id, err)
	}
	record, found, err := provider.GetLatestInventoryRecord("shein", "INV-PROVIDER-COMPAT", "us")
	if err != nil || !found || record == nil || record.ID != id || record.Stock == nil || *record.Stock != stock {
		t.Fatalf("GetLatestInventoryRecord() = %#v, %t, %v; want found persisted record", record, found, err)
	}
	missing, found, err := provider.GetLatestInventoryRecord("shein", "INV-NOT-FOUND", "us")
	if err != nil || found || missing != nil {
		t.Fatalf("GetLatestInventoryRecord(missing) = %#v, %t, %v; want nil, false, nil", missing, found, err)
	}
}

type localRuntimeInventoryRecordRow struct {
	ID                 int64      `gorm:"column:id;primaryKey;autoIncrement"`
	Platform           string     `gorm:"column:platform"`
	ProductID          string     `gorm:"column:product_id"`
	Region             string     `gorm:"column:region"`
	Stock              *int       `gorm:"column:stock"`
	StockStatus        string     `gorm:"column:stock_status"`
	IsAvailable        int16      `gorm:"column:is_available"`
	OriginalPrice      *float64   `gorm:"column:original_price"`
	CurrentPrice       *float64   `gorm:"column:current_price"`
	Currency           string     `gorm:"column:currency"`
	PriceChangePercent *float64   `gorm:"column:price_change_percent"`
	SyncSource         string     `gorm:"column:sync_source"`
	Remark             string     `gorm:"column:remark"`
	CreateTime         *time.Time `gorm:"column:create_time"`
}

func newLocalRuntimeInventoryRecordTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Table("listing_inventory_record").AutoMigrate(&localRuntimeInventoryRecordRow{}); err != nil {
		t.Fatalf("migrate inventory record: %v", err)
	}
	return db
}

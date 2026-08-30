package local

import (
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/product"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestLocalRuntimeRawJSONDataAdapterUsesResourcesWithoutCompatibilityProvider(t *testing.T) {
	db := newLocalRuntimeRawJSONTestDB(t)
	runtime := &LocalRuntime{resources: NewRuntimeResources(db, nil)}
	client := runtime.GetRawJsonDataAdapter()
	if client == nil {
		t.Fatal("GetRawJsonDataAdapter() returned nil")
	}

	id, err := client.CreateRawJsonData(&product.RawJsonCreateReq{
		TenantID:     246,
		StoreID:      986,
		ImportTaskID: 704,
		Platform:     "shein",
		Region:       "us",
		ProductID:    "RAW-RESOURCE-ONLY",
		CategoryID:   11,
		RawJsonData:  `{"name":"resource only"}`,
		Creator:      "runtime-worker",
	})
	if err != nil {
		t.Fatalf("CreateRawJsonData() error: %v", err)
	}
	if id == 0 {
		t.Fatal("CreateRawJsonData() returned zero id")
	}

	data, err := client.GetRawJsonData(&product.RawJsonReq{
		TenantID:  246,
		StoreID:   986,
		Platform:  "shein",
		Region:    "us",
		ProductID: "RAW-RESOURCE-ONLY",
	})
	if err != nil {
		t.Fatalf("GetRawJsonData() error: %v", err)
	}
	if data == nil {
		t.Fatal("GetRawJsonData() returned nil")
	}
	if data.ID != id || data.Platform != "shein" || data.ProductID != "RAW-RESOURCE-ONLY" || data.RawJSONData != `{"name":"resource only"}` {
		t.Fatalf("GetRawJsonData() = %#v, want persisted raw JSON data", data)
	}
}

func TestLocalDataProviderRawJSONCompatibilityDelegatesResourceBehavior(t *testing.T) {
	db := newLocalRuntimeRawJSONTestDB(t)
	provider := NewLocalDataProviderFromResources(NewRuntimeResources(db, nil))
	t.Cleanup(func() { _ = provider.Close() })

	id, err := provider.CreateRawJSONData(&listingadmin.RawJsonDataCreateReqDTO{
		TenantID: 246, StoreID: 986, ImportTaskID: 704, Platform: "shein", Region: "us", ProductID: "RAW-PROVIDER-COMPAT", CategoryID: 11, RawJsonData: `{"name":"provider compatibility"}`, Creator: "runtime-worker",
	})
	if err != nil || id == 0 {
		t.Fatalf("CreateRawJSONData() = %d, %v; want persisted raw JSON data", id, err)
	}

	data, err := provider.GetRawJsonData(&listingadmin.RawJsonDataReqDTO{Platform: "shein", Region: "us", ProductID: "RAW-PROVIDER-COMPAT"})
	if err != nil || data == nil || data.ID != id || data.RawJSONData != `{"name":"provider compatibility"}` {
		t.Fatalf("GetRawJsonData() = %#v, %v; want persisted compatibility data", data, err)
	}
}

func newLocalRuntimeRawJSONTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(`CREATE TABLE listing_raw_json_data (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		store_id INTEGER,
		import_task_id INTEGER,
		platform TEXT NOT NULL,
		product_id TEXT NOT NULL,
		region TEXT NOT NULL,
		category_id INTEGER,
		raw_json_data TEXT,
		status INTEGER NOT NULL DEFAULT 0,
		creator TEXT,
		updater TEXT,
		create_time DATETIME,
		update_time DATETIME,
		deleted INTEGER NOT NULL DEFAULT 0
	)`).Error; err != nil {
		t.Fatalf("migrate raw JSON data: %v", err)
	}
	return db
}

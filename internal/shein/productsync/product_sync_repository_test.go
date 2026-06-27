package productsync

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestGetStoreInfoPrefersRepository(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE listing_store (
id integer primary key autoincrement,
tenant_id integer not null,
owner_user_id text,
store_id text,
name text,
username text,
password text,
login_url text,
shop_type text,
region text,
platform text,
status integer,
deleted integer default 0
)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := db.Exec(`INSERT INTO listing_store (id, tenant_id, store_id, name, username, shop_type, region, platform, status, deleted)
VALUES (31, 41, 'S-31', 'repo-store', 'repo-user', '2', 'US', 'SHEIN', 0, 0)`).Error; err != nil {
		t.Fatalf("insert store: %v", err)
	}

	service := &productSyncServiceImpl{
		storeRepo:    listingadmin.NewGormStoreRepository(db),
		storeService: failingRuntimeStoreService{},
		logger:       logrus.NewEntry(logrus.New()),
	}

	store, err := service.getStoreInfo(context.Background(), 41, 31)
	if err != nil {
		t.Fatalf("getStoreInfo() error = %v", err)
	}
	if store == nil || store.Name != "repo-store" {
		t.Fatalf("expected repository store, got %#v", store)
	}
}

func TestGetMappingByPlatformProductIDWithoutRemoteAPI(t *testing.T) {
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.Exec(`CREATE TABLE listing_product_import_mapping (
id integer primary key autoincrement,
tenant_id integer not null,
owner_user_id text,
import_task_id integer not null,
store_id integer not null,
platform text not null,
region text not null,
product_id text not null,
parent_product_id text,
sku text,
cost_price real,
platform_product_id text,
platform_parent_product_id text,
filter_rule_id integer,
filter_rule_range text,
profit_rule_id integer,
sale_price_multiplier real,
discount_price_multiplier real,
status integer not null default 0,
remark text,
deleted integer not null default 0
)`).Error; err != nil {
		t.Fatalf("create table: %v", err)
	}
	if err := db.Exec(`INSERT INTO listing_product_import_mapping (
tenant_id, import_task_id, store_id, platform, region, product_id, sku, platform_product_id, status
) VALUES (1, 2, 3, 'SHEIN', 'US', 'ASIN-2', 'SKU-2', 'P-SKU-2', 1)`).Error; err != nil {
		t.Fatalf("insert mapping: %v", err)
	}

	service := &productSyncServiceImpl{
		mappingRepo: listingadmin.NewGormProductImportMappingRepository(db),
		logger:      logrus.NewEntry(logrus.New()),
	}

	mapping, err := service.getMappingByPlatformProductID(context.Background(), "P-SKU-2", 3)
	if err != nil {
		t.Fatalf("getMappingByPlatformProductID() error = %v", err)
	}
	if mapping == nil || mapping.ProductID != "ASIN-2" {
		t.Fatalf("expected repository mapping, got %#v", mapping)
	}
}

type failingRuntimeStoreService struct{}

func (failingRuntimeStoreService) GetStore(int64) (*listingruntime.StoreInfo, error) {
	return nil, context.Canceled
}
func (failingRuntimeStoreService) GetStorePauseStatus(int64) (bool, error) {
	return false, context.Canceled
}
func (failingRuntimeStoreService) GetStorePauseStatusDetail(int64) (*listingruntime.StorePauseStatusDetail, error) {
	return nil, context.Canceled
}
func (failingRuntimeStoreService) SetStorePauseStatus(int64, bool, string) (bool, error) {
	return false, context.Canceled
}

var _ listingruntime.StoreService = failingRuntimeStoreService{}

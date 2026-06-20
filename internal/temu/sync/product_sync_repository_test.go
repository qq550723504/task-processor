package sync

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
	managementapi "task-processor/internal/ports/managementapi"
	temuquery "task-processor/internal/temu/api/query"

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
VALUES (11, 22, 'S-11', 'repo-store', 'repo-user', '0', 'US', 'TEMU', 0, 0)`).Error; err != nil {
		t.Fatalf("insert store: %v", err)
	}

	service := &productSyncServiceImpl{
		storeRepo: listingadmin.NewGormStoreRepository(db),
		storeAPI:  failingStoreAPI{},
		logger:    logrus.NewEntry(logrus.New()),
	}

	store, err := service.getStoreInfo(context.Background(), 22, 11)
	if err != nil {
		t.Fatalf("getStoreInfo() error = %v", err)
	}
	if store == nil || store.Name != "repo-store" {
		t.Fatalf("expected repository store, got %#v", store)
	}
}

func TestGetMappingBySKUWithoutManagementClient(t *testing.T) {
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
) VALUES (1, 2, 3, 'TEMU', 'US', 'ASIN-1', 'SKU-1', 'PP-1', 1)`).Error; err != nil {
		t.Fatalf("insert mapping: %v", err)
	}

	service := &productSyncServiceImpl{
		mappingRepo: listingadmin.NewGormProductImportMappingRepository(db),
		logger:      logrus.NewEntry(logrus.New()),
	}

	mapping, err := service.getMappingBySKU(context.Background(), "SKU-1", 3)
	if err != nil {
		t.Fatalf("getMappingBySKU() error = %v", err)
	}
	if mapping == nil || mapping.ProductId != "ASIN-1" {
		t.Fatalf("expected repository mapping, got %#v", mapping)
	}
}

type failingStoreAPI struct{}

func (failingStoreAPI) GetStore(int64) (*managementapi.StoreRespDTO, error) {
	return nil, context.Canceled
}
func (failingStoreAPI) PageStores(*managementapi.StorePageReqDTO) (*managementapi.PageResult[*managementapi.StoreRespDTO], error) {
	return nil, context.Canceled
}
func (failingStoreAPI) GetStoreCookie(int64) (string, error) { return "", context.Canceled }
func (failingStoreAPI) UpdateStoreId(*managementapi.StoreIdUpdateReqDTO) (bool, error) {
	return false, context.Canceled
}
func (failingStoreAPI) UpdateStoreStatus(*managementapi.StoreStatusUpdateReqDTO) (bool, error) {
	return false, context.Canceled
}
func (failingStoreAPI) DeleteStoreCookie(int64) (bool, error) { return false, context.Canceled }
func (failingStoreAPI) SetStorePauseStatus(int64, bool, string) (bool, error) {
	return false, context.Canceled
}
func (failingStoreAPI) GetStorePauseStatus(int64) (bool, error) { return false, context.Canceled }
func (failingStoreAPI) GetStorePauseStatusDetail(int64) (*managementapi.StorePauseStatusRespDTO, error) {
	return nil, context.Canceled
}

var _ managementapi.StoreAPI = failingStoreAPI{}
var _ = temuquery.SkuQueryResponse{}

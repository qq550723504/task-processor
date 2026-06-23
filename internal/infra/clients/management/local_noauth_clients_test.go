package management

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	"task-processor/internal/pkg/types"

	miniredis "github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	_ "modernc.org/sqlite"
)

func newSQLiteProvider(t *testing.T) *LocalDataProvider {
	t.Helper()

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: dsn}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	statements := []string{
		`CREATE TABLE listing_store (
			id INTEGER PRIMARY KEY,
			tenant_id INTEGER,
			owner_user_id TEXT,
			store_id TEXT,
			name TEXT,
			username TEXT,
			password TEXT,
			login_url TEXT,
			shop_type TEXT,
			region TEXT,
			platform TEXT,
			daily_limit INTEGER,
			daily_limit_type TEXT,
			fixed_stock_count INTEGER,
			sku_generate_strategy TEXT,
			prefix TEXT,
			suffix TEXT,
			proxy TEXT,
			enable_auto_listing BOOLEAN,
			enable_auto_login BOOLEAN,
			enable_draft BOOLEAN,
			enable_auto_price BOOLEAN,
			enable_rebargain BOOLEAN,
			enable_brand_authorization BOOLEAN,
			authorized_brand_code TEXT,
			authorized_brand_name TEXT,
			temu_price_reject_strategy TEXT,
			price_type TEXT,
			remark TEXT,
			status INTEGER,
			valid_from DATETIME,
			valid_until DATETIME,
			expired BOOLEAN DEFAULT 0,
			dedicated_queue_enabled BOOLEAN,
			create_time DATETIME,
			creator TEXT,
			created_by TEXT,
			updater TEXT,
			updated_by TEXT,
			update_time DATETIME,
			deleted INTEGER DEFAULT 0
		)`,
		`CREATE TABLE listing_filter_rule (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER,
			owner_user_id TEXT,
			name TEXT,
			rule_code TEXT,
			description TEXT,
			store_id INTEGER,
			category_id INTEGER,
			price_type TEXT,
			price_min REAL,
			price_max REAL,
			stock_min INTEGER,
			rating_min REAL,
			review_count_min INTEGER,
			delivery_time_max INTEGER,
			fulfillment_type TEXT,
			status INTEGER,
			remark TEXT,
			create_time DATETIME,
			update_time DATETIME,
			deleted INTEGER DEFAULT 0
		)`,
		`CREATE TABLE listing_profit_rule (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER,
			owner_user_id TEXT,
			name TEXT,
			rule_code TEXT,
			description TEXT,
			store_id INTEGER,
			category_id INTEGER,
			sale_price_multiplier REAL,
			discount_price_multiplier REAL,
			status INTEGER,
			remark TEXT,
			create_time DATETIME,
			update_time DATETIME,
			deleted INTEGER DEFAULT 0
		)`,
		`CREATE TABLE listing_operation_strategy (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER,
			owner_user_id TEXT,
			store_id INTEGER,
			name TEXT,
			platform TEXT,
			status INTEGER,
			stock_change_threshold INTEGER,
			stock_change_action TEXT,
			out_of_stock_action TEXT,
			min_profit_rate REAL,
			low_profit_action TEXT,
			price_update_multiplier REAL,
			stock_update_ratio REAL,
			activity_enabled INTEGER,
			activity_type TEXT,
			activity_discount_rate REAL,
			activity_stock_ratio REAL,
			promotion_ratio REAL,
			activity_min_profit_rate REAL,
			activity_price_mode TEXT,
			time_limited_discount_rate REAL,
			time_limited_min_profit_rate REAL,
			time_limited_price_mode TEXT,
			time_limited_user_limit BOOLEAN,
			time_limited_user_limit_num INTEGER,
			time_limited_stock_limit BOOLEAN,
			time_limited_stock_limit_percent INTEGER,
			fixed_price_adjustment REAL,
			price_increase_threshold REAL,
			price_decrease_threshold REAL,
			price_increase_action TEXT,
			price_decrease_action TEXT,
			restore_stock_amount INTEGER,
			remark TEXT,
			create_time DATETIME,
			update_time DATETIME,
			deleted INTEGER DEFAULT 0
		)`,
		`CREATE TABLE listing_pricing_rule (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER,
			owner_user_id TEXT,
			name TEXT,
			rule_code TEXT,
			description TEXT,
			remark TEXT,
			store_id INTEGER,
			category_id INTEGER,
			price_min REAL,
			price_max REAL,
			rule_type TEXT,
			rule_value REAL,
			fixed_value REAL,
			accept_condition TEXT,
			reject_condition TEXT,
			status INTEGER,
			create_time DATETIME,
			update_time DATETIME,
			deleted INTEGER DEFAULT 0
		)`,
		`CREATE TABLE listing_product_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER,
			owner_user_id TEXT,
			source TEXT,
			import_task_id INTEGER,
			raw_json_data_id INTEGER,
			store_id INTEGER,
			category_id INTEGER,
			platform TEXT,
			region TEXT,
			parent_product_id TEXT,
			product_id TEXT,
			title TEXT,
			description TEXT,
			original_price REAL,
			special_price REAL,
			price_currency TEXT,
			stock TEXT,
			brand TEXT,
			category TEXT,
			main_image_url TEXT,
			image_urls TEXT,
			attributes TEXT,
			source_url TEXT,
			status INTEGER,
			platform_product_id TEXT,
			platform_status TEXT,
			shelf_status INTEGER,
			publish_time DATETIME,
			shelf_time DATETIME,
			last_sync_time DATETIME,
			platform_data TEXT,
			creator TEXT,
			created_by TEXT,
			create_time DATETIME,
			updater TEXT,
			updated_by TEXT,
			update_time DATETIME,
			deleted INTEGER DEFAULT 0
		)`,
		`CREATE TABLE listing_product_import_mapping (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER,
			owner_user_id TEXT,
			import_task_id INTEGER,
			store_id INTEGER,
			platform TEXT,
			region TEXT,
			product_id TEXT,
			sku TEXT,
			cost_price REAL,
			platform_product_id TEXT,
			profit_rule_id INTEGER,
			sale_price_multiplier TEXT,
			discount_price_multiplier TEXT,
			status INTEGER,
			remark TEXT,
			parent_product_id TEXT,
			platform_parent_product_id TEXT,
			filter_rule_id INTEGER,
			filter_rule_range TEXT,
			creator TEXT,
			created_by TEXT,
			create_time DATETIME,
			updater TEXT,
			updated_by TEXT,
			update_time DATETIME,
			deleted INTEGER DEFAULT 0
		)`,
		`CREATE TABLE listing_inventory_record (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			platform TEXT,
			product_id TEXT,
			region TEXT,
			stock INTEGER,
			stock_status TEXT,
			is_available BOOLEAN,
			original_price REAL,
			current_price REAL,
			currency TEXT,
			price_change_percent REAL,
			sync_source TEXT,
			remark TEXT,
			create_time DATETIME
		)`,
		`CREATE TABLE listing_product_import_task (
			id INTEGER PRIMARY KEY,
			tenant_id INTEGER,
			owner_user_id TEXT,
			store_id INTEGER,
			platform TEXT,
			region TEXT,
			category_id INTEGER,
			product_id TEXT,
			status INTEGER,
			error_message TEXT,
			reason_code TEXT,
			stage TEXT,
			retry_count INTEGER,
			max_retry_count INTEGER,
			remark TEXT,
			priority INTEGER,
			created_by TEXT,
			create_time DATETIME,
			updated_by TEXT,
			update_time DATETIME,
			published_time DATETIME,
			creator TEXT,
			updater TEXT,
			deleted INTEGER DEFAULT 0
		)`,
		`CREATE TABLE listing_raw_json_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER,
			store_id INTEGER,
			import_task_id INTEGER,
			platform TEXT,
			product_id TEXT,
			region TEXT,
			category_id INTEGER,
			raw_json_data TEXT,
			status INTEGER DEFAULT 0,
			create_time DATETIME,
			update_time DATETIME,
			creator TEXT,
			updater TEXT,
			deleted BOOLEAN DEFAULT 0
		)`,
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}

	return &LocalDataProvider{db: db}
}

func TestLocalDataProviderPageStoresFiltersByTenantAndPlatform(t *testing.T) {
	provider := newSQLiteProvider(t)
	rows := []localListingStore{
		{ID: 101, TenantID: 1, StoreID: "SHEIN-US-1", Name: "Tenant1 Shein", Platform: "shein", Region: "us", Status: 0},
		{ID: 102, TenantID: 2, StoreID: "SHEIN-US-2", Name: "Tenant2 Shein", Platform: "shein", Region: "us", Status: 0},
		{ID: 103, TenantID: 1, StoreID: "TEMU-US-1", Name: "Tenant1 Temu", Platform: "temu", Region: "us", Status: 0},
	}
	for _, row := range rows {
		if err := provider.db.Table("listing_store").Create(&row).Error; err != nil {
			t.Fatalf("seed listing_store: %v", err)
		}
	}

	page, err := provider.PageStores(&api.StorePageReqDTO{
		TenantID: 1,
		Platform: "shein",
		PageNo:   1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("PageStores() error = %v", err)
	}
	if page == nil {
		t.Fatal("PageStores() returned nil page")
	}
	if page.Total != 1 || len(page.List) != 1 {
		t.Fatalf("PageStores() = total %d len %d, want 1/1", page.Total, len(page.List))
	}
	if page.List[0].ID != 101 {
		t.Fatalf("PageStores()[0].ID = %d, want 101", page.List[0].ID)
	}
}

func TestLocalDataProviderGetStoreMapsListingStoreMetadata(t *testing.T) {
	provider := newSQLiteProvider(t)
	createdAt := time.Date(2026, 5, 15, 1, 2, 3, 0, time.UTC)
	row := localListingStore{
		ID:         201,
		TenantID:   1,
		StoreID:    "SHEIN-US-201",
		Name:       "Tenant1 Shein",
		Platform:   "shein",
		Region:     "us",
		Status:     0,
		CreateTime: &createdAt,
		Creator:    "admin",
	}
	if err := provider.db.Table("listing_store").Create(&row).Error; err != nil {
		t.Fatalf("seed listing_store: %v", err)
	}

	store, err := provider.GetStore(201)
	if err != nil {
		t.Fatalf("GetStore() error = %v", err)
	}
	if store == nil {
		t.Fatal("GetStore() returned nil")
	}
	if store.Creator != "admin" || store.CreateTime == nil || !store.CreateTime.Time.Equal(createdAt) {
		t.Fatalf("GetStore() metadata = creator %q createTime %#v", store.Creator, store.CreateTime)
	}
}

func TestLocalDataProviderPageStoresMapsDedicatedQueueEnabled(t *testing.T) {
	provider := newSQLiteProvider(t)
	if err := provider.db.Table("listing_store").Create(map[string]any{
		"id":                      976,
		"tenant_id":               322,
		"store_id":                "SHEIN-US-976",
		"name":                    "Dedicated Shein",
		"platform":                "shein",
		"region":                  "us",
		"status":                  0,
		"enable_auto_listing":     true,
		"dedicated_queue_enabled": true,
	}).Error; err != nil {
		t.Fatalf("seed listing_store: %v", err)
	}

	page, err := provider.PageStores(&api.StorePageReqDTO{
		TenantID: 322,
		Platform: "shein",
		PageNo:   1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("PageStores() error = %v", err)
	}
	if page == nil || len(page.List) != 1 {
		t.Fatalf("PageStores() returned page %#v, want one store", page)
	}
	got := page.List[0].DedicatedQueueEnabled
	if got == nil || !*got {
		t.Fatalf("PageStores()[0].DedicatedQueueEnabled = %#v, want true", got)
	}
}

func TestStoreToDTOMapsDedicatedQueueEnabled(t *testing.T) {
	enabled := true
	dto := storeToDTO(&listingadmin.Store{ID: 976, DedicatedQueueEnabled: &enabled})
	if dto == nil {
		t.Fatal("storeToDTO() returned nil")
	}
	if dto.DedicatedQueueEnabled == nil || !*dto.DedicatedQueueEnabled {
		t.Fatalf("storeToDTO().DedicatedQueueEnabled = %#v, want true", dto.DedicatedQueueEnabled)
	}
}

func TestLocalDataProviderGetFilterRuleResolvesScopedFallbacks(t *testing.T) {
	provider := newSQLiteProvider(t)
	rows := []map[string]any{
		{"tenant_id": 1, "name": "global", "rule_code": "FR-GLOBAL", "status": 1},
		{"tenant_id": 1, "name": "store", "rule_code": "FR-STORE", "store_id": 11, "status": 1},
		{"tenant_id": 1, "name": "category", "rule_code": "FR-CATEGORY", "store_id": 11, "category_id": 22, "status": 1},
	}
	for _, row := range rows {
		if err := provider.db.Table("listing_filter_rule").Create(row).Error; err != nil {
			t.Fatalf("seed listing_filter_rule: %v", err)
		}
	}

	got, err := provider.GetFilterRule(&api.FilterRuleReqDTO{TenantID: 1, StoreID: 11, CategoryID: 22})
	if err != nil {
		t.Fatalf("GetFilterRule() error = %v", err)
	}
	if got == nil || len(*got) != 1 || (*got)[0].RuleCode != "FR-CATEGORY" {
		t.Fatalf("GetFilterRule() category = %+v, want FR-CATEGORY", got)
	}

	got, err = provider.GetFilterRule(&api.FilterRuleReqDTO{TenantID: 1, StoreID: 11, CategoryID: 99})
	if err != nil {
		t.Fatalf("GetFilterRule() store fallback error = %v", err)
	}
	if got == nil || len(*got) != 1 || (*got)[0].RuleCode != "FR-STORE" {
		t.Fatalf("GetFilterRule() store fallback = %+v, want FR-STORE", got)
	}
}

func TestLocalDataProviderGetProfitRuleResolvesStoreThenGlobal(t *testing.T) {
	provider := newSQLiteProvider(t)
	rows := []map[string]any{
		{"tenant_id": 1, "name": "global", "rule_code": "PR-GLOBAL", "sale_price_multiplier": 1.1, "discount_price_multiplier": 1.05, "status": 1},
		{"tenant_id": 1, "name": "store", "rule_code": "PR-STORE", "store_id": 11, "sale_price_multiplier": 1.3, "discount_price_multiplier": 1.15, "status": 1},
	}
	for _, row := range rows {
		if err := provider.db.Table("listing_profit_rule").Create(row).Error; err != nil {
			t.Fatalf("seed listing_profit_rule: %v", err)
		}
	}

	got, err := provider.GetProfitRule(&api.ProfitRuleReqDTO{TenantID: 1, StoreID: 11})
	if err != nil {
		t.Fatalf("GetProfitRule() error = %v", err)
	}
	if got == nil || got.RuleCode != "PR-STORE" {
		t.Fatalf("GetProfitRule() = %+v, want PR-STORE", got)
	}

	got, err = provider.GetProfitRule(&api.ProfitRuleReqDTO{TenantID: 1, StoreID: 88})
	if err != nil {
		t.Fatalf("GetProfitRule() global fallback error = %v", err)
	}
	if got == nil || got.RuleCode != "PR-GLOBAL" {
		t.Fatalf("GetProfitRule() global fallback = %+v, want PR-GLOBAL", got)
	}
}

func TestLocalDataProviderGetOperationStrategyUsesRepositoryPath(t *testing.T) {
	provider := newSQLiteProvider(t)
	if err := provider.db.Table("listing_operation_strategy").Create(map[string]any{
		"tenant_id":                   1,
		"store_id":                    11,
		"name":                        "strategy",
		"platform":                    "shein",
		"status":                      0,
		"stock_change_threshold":      5,
		"stock_change_action":         "UPDATE_STOCK",
		"activity_enabled":            1,
		"activity_type":               "PROMOTION",
		"time_limited_user_limit_num": 3,
		"restore_stock_amount":        8,
	}).Error; err != nil {
		t.Fatalf("seed listing_operation_strategy: %v", err)
	}

	got, err := provider.GetOperationStrategyByStoreID(11)
	if err != nil {
		t.Fatalf("GetOperationStrategyByStoreID() error = %v", err)
	}
	if got == nil || got.StockChangeThreshold != 5 || !got.ActivityEnabled || got.RestoreStockAmount != 8 {
		t.Fatalf("GetOperationStrategyByStoreID() = %+v, want mapped strategy", got)
	}
}

func TestLocalDataProviderGetPricingRuleUsesRepositoryPath(t *testing.T) {
	provider := newSQLiteProvider(t)
	rows := []map[string]any{
		{"tenant_id": 1, "name": "latest", "rule_code": "PRICE-2", "store_id": 11, "price_min": 10, "price_max": 20, "rule_type": "ratio", "rule_value": 1.3, "status": 0},
		{"tenant_id": 1, "name": "old", "rule_code": "PRICE-1", "store_id": 11, "price_min": 1, "price_max": 9, "rule_type": "fixed", "rule_value": 2.0, "status": 0},
	}
	for _, row := range rows {
		if err := provider.db.Table("listing_pricing_rule").Create(row).Error; err != nil {
			t.Fatalf("seed listing_pricing_rule: %v", err)
		}
	}

	storeID := int64(11)
	got, err := provider.GetPricingRule(&api.PricingRuleReqDTO{StoreID: &storeID})
	if err != nil {
		t.Fatalf("GetPricingRule() error = %v", err)
	}
	if len(got) != 2 || got[0].RuleCode != "PRICE-1" && got[0].RuleCode != "PRICE-2" {
		t.Fatalf("GetPricingRule() = %+v, want two mapped rules", got)
	}
}

func TestLocalDataProviderProductDataUsesRepositoryPath(t *testing.T) {
	provider := newSQLiteProvider(t)
	now := time.Date(2026, 6, 19, 10, 0, 0, 0, time.UTC)
	if err := provider.db.Table("listing_product_data").Create(map[string]any{
		"tenant_id":           1,
		"store_id":            11,
		"platform":            "shein",
		"region":              "us",
		"product_id":          "SKU-1",
		"title":               "alpha shirt",
		"brand":               "BrandA",
		"category":            "tops",
		"platform_product_id": "SP-1",
		"shelf_status":        2,
		"attributes":          `{"color":"white"}`,
		"platform_data":       `{"spu":"123"}`,
		"create_time":         now,
		"update_time":         now,
	}).Error; err != nil {
		t.Fatalf("seed listing_product_data: %v", err)
	}

	items, err := provider.ListProductDataByStore("shein", 1, 11, nil)
	if err != nil {
		t.Fatalf("ListProductDataByStore() error = %v", err)
	}
	if len(items) != 1 || items[0].PlatformProductID != "SP-1" {
		t.Fatalf("ListProductDataByStore() = %+v, want one mapped product", items)
	}

	page, err := provider.PageProductDataByStore(&api.ProductDataListByStorePageReqDTO{
		Platform: "shein",
		TenantID: 1,
		StoreID:  11,
		Title:    "alpha",
		Brand:    "BrandA",
		Category: "top",
		PageNo:   1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("PageProductDataByStore() error = %v", err)
	}
	if page == nil || page.Total != 1 || len(page.List) != 1 {
		t.Fatalf("PageProductDataByStore() = %+v, want one result", page)
	}
}

func TestLocalDataProviderProductDataBatchWritesUseRepositoryPath(t *testing.T) {
	provider := newSQLiteProvider(t)
	count, err := provider.BatchCreateOrUpdateProductData(&api.ProductDataBatchSaveReqDTO{
		Platform: "shein",
		TenantID: 1,
		Region:   "us",
		StoreID:  11,
		Products: []api.ProductDataItemDTO{
			{
				PlatformProductID: "SP-1",
				ProductName:       "alpha shirt",
				ProductSku:        "SKU-1",
				ProductPrice:      types.FlexibleString("19.9"),
				ProductStock:      types.FlexibleString("12"),
				ProductCategory:   "tops",
				Attributes:        `{"size":"M"}`,
			},
		},
	})
	if err != nil {
		t.Fatalf("BatchCreateOrUpdateProductData() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("BatchCreateOrUpdateProductData() count = %d, want 1", count)
	}

	updated, err := provider.BatchUpdateProductAttributes(&api.ProductDataBatchUpdateAttributesReqDTO{
		Platform: "shein",
		TenantID: 1,
		Region:   "us",
		StoreID:  11,
		Products: []api.ProductAttributesItemDTO{
			{PlatformProductID: "SP-1", Attributes: `{"size":"L"}`},
		},
	})
	if err != nil {
		t.Fatalf("BatchUpdateProductAttributes() error = %v", err)
	}
	if updated != 1 {
		t.Fatalf("BatchUpdateProductAttributes() updated = %d, want 1", updated)
	}

	var row struct {
		Attributes string `gorm:"column:attributes"`
	}
	if err := provider.db.Table("listing_product_data").Where("platform_product_id = ?", "SP-1").Take(&row).Error; err != nil {
		t.Fatalf("load listing_product_data: %v", err)
	}
	if row.Attributes != `{"size":"L"}` {
		t.Fatalf("attributes = %s, want updated value", row.Attributes)
	}
}

func TestProductDataToDTO_PreservesJSONAndTimestamps(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 34, 56, 0, time.UTC)
	product := listingadmin.ProductData{
		ID:                55,
		Source:            "crawler",
		ImportTaskID:      ptrInt64(66),
		StoreID:           ptrInt64(77),
		Platform:          "shein",
		CategoryID:        ptrInt64(88),
		Region:            "us",
		ParentProductID:   "parent-1",
		ProductID:         "prod-1",
		Title:             "Demo Product",
		Description:       "desc",
		OriginalPrice:     12.34,
		SpecialPrice:      9.99,
		PriceCurrency:     "USD",
		Stock:             "123",
		Brand:             "BrandX",
		Category:          "Accessories",
		MainImageURL:      "https://img.example/1.png",
		ImageURLs:         []byte(`["a","b"]`),
		Attributes:        []byte(`{"color":"red"}`),
		SourceURL:         "https://example.com/item/1",
		Status:            3,
		RawJSONDataID:     ptrInt64(99),
		PlatformProductID: "platform-1",
		PlatformStatus:    "online",
		ShelfStatus:       ptrInt(2),
		PublishTime:       &now,
		ShelfTime:         &now,
		LastSyncTime:      &now,
		PlatformData:      []byte(`{"foo":"bar"}`),
		TenantID:          1001,
		CreateTime:        &now,
		UpdateTime:        &now,
	}

	dto := productDataToDTO(&product)
	if dto == nil {
		t.Fatal("productDataToDTO() returned nil")
	}
	if dto.ImageURLs != `["a","b"]` || dto.Attributes != `{"color":"red"}` || dto.PlatformData != `{"foo":"bar"}` {
		t.Fatalf("json fields = image:%s attrs:%s platform:%s", dto.ImageURLs, dto.Attributes, dto.PlatformData)
	}
	if dto.PublishTime == nil || dto.PublishTime.Time != now || dto.CreateTime == nil || dto.UpdateTime == nil {
		t.Fatalf("time fields were not preserved: %+v", dto)
	}
	if dto.StoreID != 77 || dto.RawJSONDataID != 99 || dto.PlatformProductID != "platform-1" {
		t.Fatalf("field mapping mismatch: %+v", dto)
	}
}

func TestLocalListingStoreToDTO_IncludesAuthorizedBrandFields(t *testing.T) {
	enabled := true
	row := localListingStore{
		ID:                       968,
		EnableBrandAuthorization: &enabled,
		AuthorizedBrandCode:      "2fd1n",
		AuthorizedBrandName:      "Logitech",
	}

	dto := row.toDTO()

	if dto.EnableBrandAuthorization == nil || !*dto.EnableBrandAuthorization {
		t.Fatalf("EnableBrandAuthorization = %#v, want true", dto.EnableBrandAuthorization)
	}
	if dto.AuthorizedBrandCode != "2fd1n" {
		t.Fatalf("AuthorizedBrandCode = %q, want 2fd1n", dto.AuthorizedBrandCode)
	}
	if dto.AuthorizedBrandName != "Logitech" {
		t.Fatalf("AuthorizedBrandName = %q, want Logitech", dto.AuthorizedBrandName)
	}
}

func TestLocalDataProviderUpdateStoreIDUsesRepositoryPath(t *testing.T) {
	provider := newSQLiteProvider(t)
	row := map[string]any{
		"tenant_id":   10,
		"store_id":    "old-id",
		"name":        "demo",
		"username":    "demo-user",
		"password":    "secret",
		"platform":    "shein",
		"shop_type":   "semi",
		"region":      "us",
		"status":      0,
		"deleted":     0,
		"create_time": time.Now(),
		"update_time": time.Now(),
	}
	if err := provider.db.Table("listing_store").Create(row).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}

	var before struct {
		ID int64 `gorm:"column:id"`
	}
	if err := provider.db.Table("listing_store").Where("store_id = ?", "old-id").Take(&before).Error; err != nil {
		t.Fatalf("load seeded store: %v", err)
	}

	ok, err := provider.UpdateStoreID(before.ID, "new-id")
	if err != nil || !ok {
		t.Fatalf("UpdateStoreID() ok=%v err=%v", ok, err)
	}

	var after struct {
		StoreID string `gorm:"column:store_id"`
	}
	if err := provider.db.Table("listing_store").Where("id = ?", before.ID).Take(&after).Error; err != nil {
		t.Fatalf("load updated store: %v", err)
	}
	if after.StoreID != "new-id" {
		t.Fatalf("store_id = %q, want new-id", after.StoreID)
	}
}

func TestLocalDataProviderUpdateStoreStatusUsesRepositoryPath(t *testing.T) {
	provider := newSQLiteProvider(t)
	row := map[string]any{
		"tenant_id":   10,
		"store_id":    "demo-id",
		"name":        "demo",
		"username":    "demo-user",
		"password":    "secret",
		"platform":    "shein",
		"shop_type":   "semi",
		"region":      "us",
		"status":      0,
		"remark":      "",
		"deleted":     0,
		"create_time": time.Now(),
		"update_time": time.Now(),
	}
	if err := provider.db.Table("listing_store").Create(row).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}

	var before struct {
		ID int64 `gorm:"column:id"`
	}
	if err := provider.db.Table("listing_store").Where("store_id = ?", "demo-id").Take(&before).Error; err != nil {
		t.Fatalf("load seeded store: %v", err)
	}

	ok, err := provider.UpdateStoreStatus(before.ID, 2, "paused by test")
	if err != nil || !ok {
		t.Fatalf("UpdateStoreStatus() ok=%v err=%v", ok, err)
	}

	var after struct {
		Status int16  `gorm:"column:status"`
		Remark string `gorm:"column:remark"`
	}
	if err := provider.db.Table("listing_store").Where("id = ?", before.ID).Take(&after).Error; err != nil {
		t.Fatalf("load updated store: %v", err)
	}
	if after.Status != 2 || after.Remark != "paused by test" {
		t.Fatalf("updated store = %+v, want status=2 remark=paused by test", after)
	}
}

func TestLocalDataProviderDeleteStoreCookieUsesRedisPath(t *testing.T) {
	provider := newSQLiteProvider(t)
	redisServer := miniredis.RunT(t)
	provider.redis = goredis.NewClient(&goredis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = provider.redis.Close()
	})

	row := map[string]any{
		"tenant_id":   10,
		"store_id":    "demo-id",
		"name":        "demo",
		"username":    "demo-user",
		"password":    "secret",
		"platform":    "shein",
		"shop_type":   "semi",
		"region":      "us",
		"status":      0,
		"deleted":     0,
		"create_time": time.Now(),
		"update_time": time.Now(),
	}
	if err := provider.db.Table("listing_store").Create(row).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}

	var store struct {
		ID       int64 `gorm:"column:id"`
		TenantID int64 `gorm:"column:tenant_id"`
	}
	if err := provider.db.Table("listing_store").Where("store_id = ?", "demo-id").Take(&store).Error; err != nil {
		t.Fatalf("load seeded store: %v", err)
	}

	ctx := context.Background()
	cookieKey := "shein:cookie:10:" + strconv.FormatInt(store.ID, 10)
	if err := provider.redis.Set(ctx, cookieKey, `[{"name":"sid"}]`, time.Hour).Err(); err != nil {
		t.Fatalf("seed cookie: %v", err)
	}

	ok, err := provider.DeleteStoreCookie(store.ID)
	if err != nil || !ok {
		t.Fatalf("DeleteStoreCookie() ok=%v err=%v", ok, err)
	}
	if provider.redis.Exists(ctx, cookieKey).Val() != 0 {
		t.Fatal("cookie key still exists after DeleteStoreCookie()")
	}
}

func TestLocalDataProviderDeleteStoreCookieSkipsRecentLogin(t *testing.T) {
	provider := newSQLiteProvider(t)
	redisServer := miniredis.RunT(t)
	provider.redis = goredis.NewClient(&goredis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = provider.redis.Close()
	})

	row := map[string]any{
		"tenant_id":   10,
		"store_id":    "demo-id",
		"name":        "demo",
		"username":    "demo-user",
		"password":    "secret",
		"platform":    "shein",
		"shop_type":   "semi",
		"region":      "us",
		"status":      0,
		"deleted":     0,
		"create_time": time.Now(),
		"update_time": time.Now(),
	}
	if err := provider.db.Table("listing_store").Create(row).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}

	var store struct {
		ID       int64 `gorm:"column:id"`
		TenantID int64 `gorm:"column:tenant_id"`
	}
	if err := provider.db.Table("listing_store").Where("store_id = ?", "demo-id").Take(&store).Error; err != nil {
		t.Fatalf("load seeded store: %v", err)
	}

	ctx := context.Background()
	cookieKey := "shein:cookie:10:" + strconv.FormatInt(store.ID, 10)
	lastLoginKey := "shein:last_login_time:10:" + strconv.FormatInt(store.ID, 10)
	if err := provider.redis.Set(ctx, cookieKey, `[{"name":"sid"}]`, time.Hour).Err(); err != nil {
		t.Fatalf("seed cookie: %v", err)
	}
	if err := provider.redis.Set(ctx, lastLoginKey, time.Now().Unix(), time.Hour).Err(); err != nil {
		t.Fatalf("seed last login: %v", err)
	}

	ok, err := provider.DeleteStoreCookie(store.ID)
	if err != nil {
		t.Fatalf("DeleteStoreCookie() error = %v", err)
	}
	if ok {
		t.Fatal("DeleteStoreCookie() ok = true, want false for recent login")
	}
	if provider.redis.Exists(ctx, cookieKey).Val() != 1 {
		t.Fatal("cookie key should remain after recent login guard")
	}
}

func TestLocalDataProviderGetRawJSONDataIgnoresDeletedRows(t *testing.T) {
	provider := newSQLiteProvider(t)
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC)

	rows := []map[string]any{
		{
			"platform":      "amazon",
			"product_id":    "B0TEST123",
			"region":        "us",
			"raw_json_data": `{"source":"active"}`,
			"create_time":   now.Add(-2 * time.Hour),
			"update_time":   now.Add(-2 * time.Hour),
			"creator":       "tester",
			"updater":       "tester",
			"deleted":       false,
		},
		{
			"platform":      "amazon",
			"product_id":    "B0TEST123",
			"region":        "us",
			"raw_json_data": `{"source":"deleted"}`,
			"create_time":   now.Add(-1 * time.Hour),
			"update_time":   now.Add(-1 * time.Hour),
			"creator":       "tester",
			"updater":       "tester",
			"deleted":       true,
		},
	}
	for _, row := range rows {
		if err := provider.db.Table("listing_raw_json_data").Create(row).Error; err != nil {
			t.Fatalf("seed listing_raw_json_data: %v", err)
		}
	}

	got, err := provider.GetRawJSONData(&api.RawJsonDataReqDTO{
		Platform:  "amazon",
		ProductID: "B0TEST123",
		Region:    "us",
	})
	if err != nil {
		t.Fatalf("GetRawJSONData() error = %v", err)
	}
	if got == nil {
		t.Fatal("GetRawJSONData() returned nil")
	}
	if got.RawJSONData != `{"source":"active"}` {
		t.Fatalf("GetRawJSONData() raw_json_data = %s, want active row", got.RawJSONData)
	}
}

func TestLocalDataProviderGetRawJSONDataSupportsSmallintDeletedColumn(t *testing.T) {
	provider := newSQLiteProvider(t)
	if err := provider.db.Exec(`DROP TABLE listing_raw_json_data`).Error; err != nil {
		t.Fatalf("drop listing_raw_json_data: %v", err)
	}
	if err := provider.db.Exec(`
		CREATE TABLE listing_raw_json_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER,
			store_id INTEGER,
			import_task_id INTEGER,
			product_id TEXT,
			platform TEXT,
			region TEXT,
			category_id INTEGER,
			raw_json_data TEXT,
			status INTEGER DEFAULT 0,
			creator TEXT,
			create_time DATETIME,
			updater TEXT,
			update_time DATETIME,
			deleted INTEGER DEFAULT 0
		)
	`).Error; err != nil {
		t.Fatalf("create listing_raw_json_data with smallint-like deleted: %v", err)
	}

	if _, err := provider.CreateRawJSONData(&api.RawJsonDataCreateReqDTO{
		TenantID:     10,
		StoreID:      20,
		ImportTaskID: 30,
		Platform:     "amazon",
		ProductID:    "B0SMALLINT",
		Region:       "us",
		CategoryID:   40,
		RawJsonData:  `{"asin":"B0SMALLINT"}`,
		Creator:      "tester",
	}); err != nil {
		t.Fatalf("CreateRawJSONData() error = %v", err)
	}

	got, err := provider.GetRawJSONData(&api.RawJsonDataReqDTO{
		Platform:  "amazon",
		ProductID: "B0SMALLINT",
		Region:    "us",
	})
	if err != nil {
		t.Fatalf("GetRawJSONData() error = %v", err)
	}
	if got == nil || got.RawJSONData == "" {
		t.Fatalf("GetRawJSONData() = %+v, want stored row", got)
	}
	if got.TaskID != 30 {
		t.Fatalf("GetRawJSONData().TaskID = %d, want 30", got.TaskID)
	}
}

func TestProductImportMappingAPIClient_LocalProvider(t *testing.T) {
	provider := newSQLiteProvider(t)
	client := &ProductImportMappingAPIClient{
		ManagementAPIClient: NewManagementAPIClientWithBaseURL("http://127.0.0.1:1"),
		localDataProvider:   provider,
	}

	sku := "SKU-1"
	platformProductID := "PP-1"
	parentProductID := "PARENT-1"
	saleMultiplier := "1.35"
	discountMultiplier := "1.10"
	remark := "draft"
	status := int16(6)
	id, err := client.CreateProductImportMapping(&api.ProductImportMappingCreateReqDTO{
		TenantID:                1,
		ImportTaskId:            1001,
		StoreId:                 2002,
		Platform:                "shein",
		Region:                  "us",
		ProductId:               "B0TEST",
		Sku:                     &sku,
		PlatformProductId:       &platformProductID,
		ParentProductId:         &parentProductID,
		SalePriceMultiplier:     &saleMultiplier,
		DiscountPriceMultiplier: &discountMultiplier,
		Remark:                  &remark,
		Status:                  &status,
	})
	if err != nil {
		t.Fatalf("CreateProductImportMapping() error = %v", err)
	}
	if id == 0 {
		t.Fatal("CreateProductImportMapping() id should not be 0")
	}

	mapping, err := client.GetProductImportMappingByPlatformProductId(&api.ProductImportMappingGetReqDTO{PlatformProductId: platformProductID})
	if err != nil {
		t.Fatalf("GetProductImportMappingByPlatformProductId() error = %v", err)
	}
	if mapping == nil || mapping.ImportTaskId != 1001 {
		t.Fatalf("GetProductImportMappingByPlatformProductId() = %+v", mapping)
	}
	if mapping.SalePriceMultiplier == nil || *mapping.SalePriceMultiplier != 1.35 {
		t.Fatalf("SalePriceMultiplier = %+v, want 1.35", mapping.SalePriceMultiplier)
	}

	updatedPlatformProductID := "PP-2"
	updateID := id
	if err := client.UpdateProductImportMapping(&api.ProductImportMappingCreateReqDTO{
		ID:                &updateID,
		TenantID:          1,
		ImportTaskId:      1001,
		StoreId:           2002,
		Platform:          "shein",
		Region:            "us",
		ProductId:         "B0TEST",
		Sku:               &sku,
		PlatformProductId: &updatedPlatformProductID,
		Status:            &status,
	}); err != nil {
		t.Fatalf("UpdateProductImportMapping() error = %v", err)
	}

	byTaskAndSKU, err := client.GetProductImportMappingByTaskAndSku(1001, sku)
	if err != nil {
		t.Fatalf("GetProductImportMappingByTaskAndSku() error = %v", err)
	}
	if byTaskAndSKU == nil || byTaskAndSKU.PlatformProductId == nil || *byTaskAndSKU.PlatformProductId != updatedPlatformProductID {
		t.Fatalf("GetProductImportMappingByTaskAndSku() = %+v", byTaskAndSKU)
	}

	bySKU, err := client.GetProductImportMappingBySku(&api.ProductImportMappingGetBySkuReqDTO{Sku: sku, StoreId: 2002})
	if err != nil {
		t.Fatalf("GetProductImportMappingBySku() error = %v", err)
	}
	if bySKU == nil || bySKU.PlatformProductId == nil || *bySKU.PlatformProductId != updatedPlatformProductID {
		t.Fatalf("GetProductImportMappingBySku() = %+v", bySKU)
	}

	byPlatformAndStore, err := client.GetProductImportMappingByPlatformProductIdAndStore(&api.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO{
		PlatformProductId: updatedPlatformProductID,
		StoreId:           2002,
	})
	if err != nil {
		t.Fatalf("GetProductImportMappingByPlatformProductIdAndStore() error = %v", err)
	}
	if byPlatformAndStore == nil || byPlatformAndStore.Sku == nil || *byPlatformAndStore.Sku != sku {
		t.Fatalf("GetProductImportMappingByPlatformProductIdAndStore() = %+v", byPlatformAndStore)
	}

	exists, err := client.CheckProductExists(&api.ProductImportMappingCheckReqDTO{
		StoreId:   2002,
		Platform:  "shein",
		Region:    "us",
		ProductId: "B0TEST",
	})
	if err != nil {
		t.Fatalf("CheckProductExists() error = %v", err)
	}
	if !exists {
		t.Fatal("CheckProductExists() = false, want true")
	}
}

func TestInventoryRecordAPIClient_LocalProvider(t *testing.T) {
	provider := newSQLiteProvider(t)
	client := &InventoryRecordAPIClient{
		ManagementAPIClient: NewManagementAPIClientWithBaseURL("http://127.0.0.1:1"),
		localDataProvider:   provider,
	}

	stock1 := 10
	stock2 := 15
	if _, err := client.CreateInventoryRecord(&api.InventoryRecordCreateReqDTO{
		Platform:    "shein",
		ProductId:   "P-1",
		Region:      "us",
		Stock:       &stock1,
		IsAvailable: true,
		SyncSource:  "initial",
	}); err != nil {
		t.Fatalf("CreateInventoryRecord(first) error = %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := client.CreateInventoryRecord(&api.InventoryRecordCreateReqDTO{
		Platform:    "shein",
		ProductId:   "P-1",
		Region:      "us",
		Stock:       &stock2,
		IsAvailable: true,
		SyncSource:  "latest",
	}); err != nil {
		t.Fatalf("CreateInventoryRecord(second) error = %v", err)
	}

	record, err := client.GetLatestInventoryRecord("shein", "P-1", "us")
	if err != nil {
		t.Fatalf("GetLatestInventoryRecord() error = %v", err)
	}
	if record == nil || record.Stock == nil || *record.Stock != stock2 {
		t.Fatalf("GetLatestInventoryRecord() = %+v, want stock %d", record, stock2)
	}
}

func TestImportTaskAPIClient_LocalProvider(t *testing.T) {
	provider := newSQLiteProvider(t)
	now := time.Now()
	rows := []localImportTaskRow{
		{ID: 1, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "A", Status: model.TaskStatusPending.Int16(), Priority: 10, CreateTime: now, UpdateTime: now},
		{ID: 2, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "B", Status: model.TaskStatusPendingRetry.Int16(), Priority: 20, CreateTime: now, UpdateTime: now},
		{ID: 3, TenantID: 10, StoreID: 100, Platform: "shein", Region: "us", ProductID: "C", Status: model.TaskStatusPublished.Int16(), Priority: 30, CreateTime: now, UpdateTime: now},
	}
	for _, row := range rows {
		if err := provider.db.Table("listing_product_import_task").Create(&row).Error; err != nil {
			t.Fatalf("seed import task: %v", err)
		}
	}

	client := &ImportTaskAPIClient{
		ManagementAPIClient: NewManagementAPIClientWithBaseURL("http://127.0.0.1:1"),
		localDataProvider:   provider,
	}

	tasks, err := client.GetPendingAndRetryTasks(10, 10, []int64{100})
	if err != nil {
		t.Fatalf("GetPendingAndRetryTasks() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("GetPendingAndRetryTasks() count = %d, want 2", len(tasks))
	}

	expected := model.TaskStatusPending.Int16()
	if err := client.UpdateTaskStatus(&api.ProductImportTaskUpdateReqDTO{
		ID:                    1,
		Status:                model.TaskStatusProcessing.Int16(),
		ExpectedCurrentStatus: &expected,
	}); err != nil {
		t.Fatalf("UpdateTaskStatus() error = %v", err)
	}

	var updated localImportTaskRow
	if err := provider.db.Table("listing_product_import_task").Where("id = ?", 1).Take(&updated).Error; err != nil {
		t.Fatalf("reload updated row: %v", err)
	}
	if updated.Status != model.TaskStatusProcessing.Int16() {
		t.Fatalf("updated status = %d, want %d", updated.Status, model.TaskStatusProcessing.Int16())
	}

	badExpected := model.TaskStatusPending.Int16()
	err = client.UpdateTaskStatus(&api.ProductImportTaskUpdateReqDTO{
		ID:                    1,
		Status:                model.TaskStatusPublished.Int16(),
		ExpectedCurrentStatus: &badExpected,
	})
	if err == nil || !strings.Contains(err.Error(), "管理端拒绝更新任务状态") {
		t.Fatalf("UpdateTaskStatus() mismatch error = %v", err)
	}
}

func TestImportTaskAPIClient_LocalProviderReturnsPublishedTime(t *testing.T) {
	provider := newSQLiteProvider(t)
	now := time.Now()
	publishedAt := now.Add(2 * time.Minute)
	row := localImportTaskRow{
		ID:            11,
		TenantID:      10,
		StoreID:       976,
		Platform:      "shein",
		Region:        "us",
		ProductID:     "draft-product",
		Status:        model.TaskStatusDraft.Int16(),
		Priority:      10,
		CreateTime:    now,
		UpdateTime:    publishedAt,
		PublishedTime: &publishedAt,
	}
	if err := provider.db.Table("listing_product_import_task").Create(&row).Error; err != nil {
		t.Fatalf("seed import task: %v", err)
	}

	client := &ImportTaskAPIClient{
		ManagementAPIClient: NewManagementAPIClientWithBaseURL("http://127.0.0.1:1"),
		localDataProvider:   provider,
	}
	task, err := client.GetTaskByID(row.ID)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}
	if task == nil {
		t.Fatal("GetTaskByID() = nil, want task")
	}
	if task.PublishedTime != publishedAt.UnixMilli() {
		t.Fatalf("PublishedTime = %d, want %d", task.PublishedTime, publishedAt.UnixMilli())
	}
}

func TestClientManagerRuntimeUsesLocalProviderWhenManagementHTTPUnavailable(t *testing.T) {
	provider := newSQLiteProvider(t)
	redisServer, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(redisServer.Close)
	provider.redis = goredis.NewClient(&goredis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = provider.redis.Close() })

	now := time.Now()
	enableAutoListing := true
	enableDraft := true
	dailyLimit := 300
	store := localListingStore{
		ID:                976,
		TenantID:          322,
		StoreID:           "976",
		Name:              "SHEIN 976",
		Username:          "store-976",
		Platform:          "shein",
		Region:            "us",
		Status:            0,
		EnableAutoListing: &enableAutoListing,
		EnableDraft:       &enableDraft,
		DailyLimit:        &dailyLimit,
		DailyLimitType:    "SPU",
		CreateTime:        &now,
	}
	if err := provider.db.Table("listing_store").Create(&store).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}
	seed := localImportTaskRow{
		ID:            21,
		TenantID:      322,
		StoreID:       976,
		Platform:      "shein",
		Region:        "us",
		ProductID:     "runtime-product",
		Status:        model.TaskStatusProcessing.Int16(),
		Priority:      10,
		CreateTime:    now,
		UpdateTime:    now,
		MaxRetryCount: 3,
	}
	if err := provider.db.Table("listing_product_import_task").Create(&seed).Error; err != nil {
		t.Fatalf("seed import task: %v", err)
	}

	manager := NewClientManager(&config.ManagementConfig{BaseURL: "http://127.0.0.1:1"})
	manager.SetLocalDataProvider(provider)

	storeInfo, err := manager.GetRuntimeStoreService().GetStore(store.ID)
	if err != nil {
		t.Fatalf("GetRuntimeStoreService().GetStore() error = %v", err)
	}
	if storeInfo == nil || storeInfo.ID != store.ID || storeInfo.EnableAutoListing == nil || !*storeInfo.EnableAutoListing {
		t.Fatalf("GetRuntimeStoreService().GetStore() = %+v, want enabled store", storeInfo)
	}

	dailyClient := manager.GetDailyListingCountClient()
	date := "2026-06-24"
	quota, err := dailyClient.TryConsumeDailyQuota(&api.TryConsumeDailyQuotaReqDTO{
		TenantID:  seed.TenantID,
		StoreID:   seed.StoreID,
		Date:      date,
		Increment: 2,
		Limit:     int64(dailyLimit),
	})
	if err != nil {
		t.Fatalf("TryConsumeDailyQuota() error = %v", err)
	}
	if quota == nil || !quota.Allowed || quota.NewCount != 2 {
		t.Fatalf("TryConsumeDailyQuota() = %+v, want allowed count 2", quota)
	}
	rolledBack, err := dailyClient.RollbackDailyQuota(&api.RollbackDailyQuotaReqDTO{
		TenantID:  seed.TenantID,
		StoreID:   seed.StoreID,
		Date:      date,
		Decrement: 1,
	})
	if err != nil {
		t.Fatalf("RollbackDailyQuota() error = %v", err)
	}
	if rolledBack != 1 {
		t.Fatalf("RollbackDailyQuota() = %d, want 1", rolledBack)
	}

	task, err := manager.GetRuntimeImportTask(seed.ID)
	if err != nil {
		t.Fatalf("GetRuntimeImportTask() error = %v", err)
	}
	if task == nil || task.ID != seed.ID || task.ProductID != seed.ProductID {
		t.Fatalf("GetRuntimeImportTask() = %+v, want seeded task", task)
	}

	expected := model.TaskStatusProcessing.Int16()
	if err := manager.UpdateRuntimeTaskStatus(&listingruntime.TaskStatusUpdate{
		ID:                    seed.ID,
		Status:                model.TaskStatusDraft.Int16(),
		ExpectedCurrentStatus: &expected,
	}); err != nil {
		t.Fatalf("UpdateRuntimeTaskStatus() error = %v", err)
	}
	var updated localImportTaskRow
	if err := provider.db.Table("listing_product_import_task").Where("id = ?", seed.ID).Take(&updated).Error; err != nil {
		t.Fatalf("reload import task: %v", err)
	}
	if updated.Status != model.TaskStatusDraft.Int16() {
		t.Fatalf("updated status = %d, want draft", updated.Status)
	}
	if updated.PublishedTime == nil {
		t.Fatal("published_time is nil, want local status update to set completion time")
	}

	sku := "SKU-976"
	platformProductID := "SPU-976"
	mappingStatus := model.TaskStatusPublished.Int16()
	mappingID, err := manager.CreateRuntimeProductImportMapping(context.Background(), &listingruntime.ProductImportMappingUpsert{
		TenantID:          seed.TenantID,
		ImportTaskID:      seed.ID,
		StoreID:           seed.StoreID,
		Platform:          seed.Platform,
		Region:            seed.Region,
		ProductID:         seed.ProductID,
		SKU:               &sku,
		PlatformProductID: &platformProductID,
		Status:            &mappingStatus,
	})
	if err != nil {
		t.Fatalf("CreateRuntimeProductImportMapping() error = %v", err)
	}
	if mappingID == 0 {
		t.Fatal("CreateRuntimeProductImportMapping() id = 0, want local row id")
	}

	mapping, err := manager.FindRuntimeProductImportMappingByTaskAndSKU(context.Background(), seed.ID, sku)
	if err != nil {
		t.Fatalf("FindRuntimeProductImportMappingByTaskAndSKU() error = %v", err)
	}
	if mapping == nil || mapping.PlatformProductID == nil || *mapping.PlatformProductID != platformProductID {
		t.Fatalf("FindRuntimeProductImportMappingByTaskAndSKU() = %+v, want platform product id %s", mapping, platformProductID)
	}
	exists, err := manager.RuntimePublishedProductExists(context.Background(), seed.StoreID, seed.Platform, seed.Region, seed.ProductID)
	if err != nil {
		t.Fatalf("RuntimePublishedProductExists() error = %v", err)
	}
	if !exists {
		t.Fatal("RuntimePublishedProductExists() = false, want true")
	}
}

func TestTaskRPCAPIClient_LocalProvider(t *testing.T) {
	provider := newSQLiteProvider(t)
	client := &TaskRPCAPIClient{
		ManagementAPIClient: NewManagementAPIClientWithBaseURL("http://127.0.0.1:1"),
		localProvider:       NewLocalTaskRPCProvider(provider),
	}

	submitResp, err := client.SubmitTask(&api.TaskSubmitReqDTO{
		TaskID:           9001,
		TenantID:         11,
		StoreID:          22,
		Platform:         "temu",
		Region:           "us",
		ProductID:        "SKU-9001",
		TaskType:         "publish",
		BusinessPriority: 6,
		MaxRetries:       4,
	})
	if err != nil {
		t.Fatalf("SubmitTask() error = %v", err)
	}
	if !submitResp.Success || submitResp.TaskID != 9001 {
		t.Fatalf("SubmitTask() = %+v", submitResp)
	}

	statusResp, err := client.GetTaskStatus(9001)
	if err != nil {
		t.Fatalf("GetTaskStatus() error = %v", err)
	}
	if statusResp == nil || statusResp.StatusKey != "PENDING" {
		t.Fatalf("GetTaskStatus() = %+v", statusResp)
	}

	if err := provider.db.Table("listing_product_import_task").Where("id = ?", 9001).Updates(map[string]any{
		"status":      model.TaskStatusCrawlFailed.Int16(),
		"update_time": time.Now(),
	}).Error; err != nil {
		t.Fatalf("prepare retry row: %v", err)
	}

	retryResp, err := client.RetryTask(9001)
	if err != nil {
		t.Fatalf("RetryTask() error = %v", err)
	}
	if !retryResp.Success || retryResp.StatusKey != "PENDING_RETRY" {
		t.Fatalf("RetryTask() = %+v", retryResp)
	}

	batchResp, err := client.SubmitBatchTasks(&api.TaskBatchSubmitReqDTO{
		Tasks: []api.TaskSubmitReqDTO{
			{TaskID: 9002, TenantID: 11, StoreID: 22, Platform: "temu", Region: "us", ProductID: "SKU-9002", TaskType: "publish"},
			{TaskID: 9003, TenantID: 11, StoreID: 22, Platform: "temu", Region: "us", ProductID: "SKU-9003", TaskType: "publish"},
		},
	})
	if err != nil {
		t.Fatalf("SubmitBatchTasks() error = %v", err)
	}
	if batchResp.SuccessCount != 2 || batchResp.FailureCount != 0 {
		t.Fatalf("SubmitBatchTasks() = %+v", batchResp)
	}

	stats, err := client.GetQueueStats()
	if err != nil {
		t.Fatalf("GetQueueStats() error = %v", err)
	}
	if !strings.Contains(stats, `"source":"local-db"`) {
		t.Fatalf("GetQueueStats() = %s", stats)
	}
}

func TestImportTaskAPIClient_GetTaskByID_LocalProvider(t *testing.T) {
	provider := newSQLiteProvider(t)
	row := localImportTaskRow{
		ID:            9101,
		TenantID:      12,
		StoreID:       34,
		Platform:      "shein",
		Region:        "us",
		CategoryID:    56,
		ProductID:     "B0BPF6V5V6",
		Status:        model.TaskStatusPending.Int16(),
		RetryCount:    1,
		MaxRetryCount: 3,
		Priority:      9,
		CreateTime:    time.Now(),
		UpdateTime:    time.Now(),
		Creator:       "tester",
		Updater:       "tester",
	}
	if err := provider.db.Table("listing_product_import_task").Create(&row).Error; err != nil {
		t.Fatalf("insert import task row: %v", err)
	}

	client := &ImportTaskAPIClient{
		ManagementAPIClient: NewManagementAPIClientWithBaseURL("http://127.0.0.1:1"),
		localDataProvider:   provider,
	}

	task, err := client.GetTaskByID(9101)
	if err != nil {
		t.Fatalf("GetTaskByID() error = %v", err)
	}
	if task == nil || task.ID != 9101 || task.ProductID != "B0BPF6V5V6" || task.StatusKey != "PENDING" {
		t.Fatalf("GetTaskByID() = %+v", task)
	}
}

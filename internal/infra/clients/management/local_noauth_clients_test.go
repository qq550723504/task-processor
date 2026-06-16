package management

import (
	"strings"
	"testing"
	"time"

	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"

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
			create_time DATETIME,
			creator TEXT
		)`,
		`CREATE TABLE listing_product_import_mapping (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id INTEGER,
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
			create_time DATETIME,
			update_time DATETIME
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
			create_time DATETIME,
			update_time DATETIME,
			creator TEXT,
			updater TEXT
		)`,
		`CREATE TABLE listing_raw_json_data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			platform TEXT,
			product_id TEXT,
			region TEXT,
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
			product_id TEXT,
			platform TEXT,
			region TEXT,
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
		Platform:    "amazon",
		ProductID:   "B0SMALLINT",
		Region:      "us",
		RawJsonData: `{"asin":"B0SMALLINT"}`,
		Creator:     "tester",
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

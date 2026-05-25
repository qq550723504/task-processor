package listingadmin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

func TestStoreStatisticsHandlerListsAutoListingStoresWithinTenant(t *testing.T) {
	router := newStoreStatisticsTestRouter(t)
	trueValue := true
	falseValue := false
	limit := 10
	seedStore(t, router.db, listingStore{
		ID:                1,
		TenantID:          101,
		StoreID:           "SHEIN-US",
		Name:              "SHEIN US",
		Username:          "shein-us",
		Password:          "secret",
		Platform:          "SHEIN",
		ShopType:          "semi",
		Region:            "US",
		DailyLimit:        &limit,
		DailyLimitType:    "fixed",
		EnableAutoListing: &trueValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})
	seedStore(t, router.db, listingStore{
		ID:                2,
		TenantID:          101,
		Name:              "Manual Store",
		Username:          "manual",
		Password:          "secret",
		Platform:          "SHEIN",
		ShopType:          "semi",
		EnableAutoListing: &falseValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})
	seedStore(t, router.db, listingStore{
		ID:                3,
		TenantID:          202,
		Name:              "Other Tenant",
		Username:          "other",
		Password:          "secret",
		Platform:          "TEMU",
		ShopType:          "semi",
		EnableAutoListing: &trueValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})
	seedStatisticsImportTask(t, router.db, listingProductImportTask{TenantID: 101, StoreID: 1, Platform: "SHEIN", Region: "US", ProductID: "P1", Status: 0, CreateTime: timePtr(time.Date(2026, 5, 15, 8, 0, 0, 0, time.UTC))})
	seedStatisticsImportTask(t, router.db, listingProductImportTask{TenantID: 101, StoreID: 1, Platform: "SHEIN", Region: "US", ProductID: "P2", Status: 1, CreateTime: timePtr(time.Date(2026, 5, 15, 8, 30, 0, 0, time.UTC))})
	seedStatisticsImportTask(t, router.db, listingProductImportTask{TenantID: 101, StoreID: 1, Platform: "SHEIN", Region: "US", ProductID: "P3", Status: 5, CreateTime: timePtr(time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC))})
	seedStatisticsImportTask(t, router.db, listingProductImportTask{TenantID: 101, StoreID: 1, Platform: "SHEIN", Region: "US", ProductID: "P4", Status: 10, CreateTime: timePtr(time.Date(2026, 5, 15, 9, 30, 0, 0, time.UTC))})
	seedStatisticsImportTask(t, router.db, listingProductImportTask{TenantID: 101, StoreID: 1, Platform: "SHEIN", Region: "US", ProductID: "P5", Status: 2, CreateTime: timePtr(time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC))})

	req := httptest.NewRequest(http.MethodGet, "/store-statistics?date=2026-05-15", nil)
	req.Header.Set("X-Tenant-ID", "101")
	req.Header.Set("X-User-ID", "user-101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /store-statistics = %d, body=%s", resp.Code, resp.Body.String())
	}
	var items []StoreStatistics
	if err := json.Unmarshal(resp.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %+v, want one auto listing store for tenant 101", items)
	}
	got := items[0]
	if got.Name != "SHEIN US" || got.CompletedCount != 1 || got.RemainingCount != 2 || got.QueuedCount != 1 || got.HoldCount != 1 {
		t.Fatalf("statistics = %+v, want aggregated counts", got)
	}
	if got.ProgressPercentage != 10 {
		t.Fatalf("progress = %v, want 10", got.ProgressPercentage)
	}
}

func TestStoreStatisticsHandlerOwnerScopeFiltersStoresByUser(t *testing.T) {
	t.Cleanup(SetOwnerScopeRequiredForTesting(true))

	router := newStoreStatisticsTestRouter(t)
	trueValue := true
	seedStore(t, router.db, listingStore{
		ID:                1,
		TenantID:          101,
		OwnerUserID:       "user-a",
		CreatedBy:         "user-a",
		UpdatedBy:         "user-a",
		Name:              "Owned by A",
		Username:          "a",
		Password:          "secret",
		Platform:          "SHEIN",
		ShopType:          "semi",
		EnableAutoListing: &trueValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})
	seedStore(t, router.db, listingStore{
		ID:                2,
		TenantID:          101,
		OwnerUserID:       "user-b",
		CreatedBy:         "user-b",
		UpdatedBy:         "user-b",
		Name:              "Owned by B",
		Username:          "b",
		Password:          "secret",
		Platform:          "SHEIN",
		ShopType:          "semi",
		EnableAutoListing: &trueValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})

	req := httptest.NewRequest(http.MethodGet, "/store-statistics", nil)
	req.Header.Set("X-Tenant-ID", "101")
	req.Header.Set("X-User-ID", "user-a")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /store-statistics = %d, body=%s", resp.Code, resp.Body.String())
	}
	var items []StoreStatistics
	if err := json.Unmarshal(resp.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Owned by A" {
		t.Fatalf("statistics items = %+v, want only user-a store", items)
	}
}

func TestStoreStatisticsHandlerPlatformAdminBypassesOwnerScope(t *testing.T) {
	t.Cleanup(SetOwnerScopeRequiredForTesting(true))

	router := newStoreStatisticsTestRouter(t)
	trueValue := true
	seedStore(t, router.db, listingStore{
		ID:                1,
		TenantID:          101,
		OwnerUserID:       "user-a",
		CreatedBy:         "user-a",
		UpdatedBy:         "user-a",
		Name:              "Owned by A",
		Username:          "a",
		Password:          "secret",
		Platform:          "SHEIN",
		ShopType:          "semi",
		EnableAutoListing: &trueValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})
	seedStore(t, router.db, listingStore{
		ID:                2,
		TenantID:          101,
		OwnerUserID:       "user-b",
		CreatedBy:         "user-b",
		UpdatedBy:         "user-b",
		Name:              "Owned by B",
		Username:          "b",
		Password:          "secret",
		Platform:          "SHEIN",
		ShopType:          "semi",
		EnableAutoListing: &trueValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})

	req := httptest.NewRequest(http.MethodGet, "/store-statistics", nil)
	req.Header.Set("X-Tenant-ID", "101")
	req.Header.Set("X-User-ID", "platform-admin")
	req.Header.Set("X-User-Roles", "platform_admin")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /store-statistics = %d, body=%s", resp.Code, resp.Body.String())
	}
	var items []StoreStatistics
	if err := json.Unmarshal(resp.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("statistics items = %+v, want both stores", items)
	}
}

func TestStoreStatisticsHandlerAcceptsTenantIDQueryFallback(t *testing.T) {
	router := newStoreStatisticsTestRouter(t)
	trueValue := true
	seedStore(t, router.db, listingStore{
		ID:                1,
		TenantID:          101,
		Name:              "Query Tenant Store",
		Username:          "query-tenant",
		Password:          "secret",
		Platform:          "SHEIN",
		ShopType:          "semi",
		EnableAutoListing: &trueValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})

	req := httptest.NewRequest(http.MethodGet, "/store-statistics?tenant_id=101", nil)
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /store-statistics?tenant_id=101 = %d, body=%s", resp.Code, resp.Body.String())
	}
	var items []StoreStatistics
	if err := json.Unmarshal(resp.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items) != 1 || items[0].Name != "Query Tenant Store" {
		t.Fatalf("statistics items = %+v, want query-tenant scoped store", items)
	}
}

func TestStoreStatisticsHandlerFiltersTaskCountsByDate(t *testing.T) {
	router := newStoreStatisticsTestRouter(t)
	trueValue := true
	limit := 10
	seedStore(t, router.db, listingStore{
		ID:                1,
		TenantID:          101,
		StoreID:           "SHEIN-US",
		Name:              "SHEIN US",
		Username:          "shein-us",
		Password:          "secret",
		Platform:          "SHEIN",
		ShopType:          "semi",
		Region:            "US",
		DailyLimit:        &limit,
		DailyLimitType:    "fixed",
		EnableAutoListing: &trueValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})
	seedStatisticsImportTask(t, router.db, listingProductImportTask{TenantID: 101, StoreID: 1, Platform: "SHEIN", Region: "US", ProductID: "D1", Status: 2, CreateTime: timePtr(time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC))})
	seedStatisticsImportTask(t, router.db, listingProductImportTask{TenantID: 101, StoreID: 1, Platform: "SHEIN", Region: "US", ProductID: "D2", Status: 2, CreateTime: timePtr(time.Date(2026, 5, 16, 9, 0, 0, 0, time.UTC))})
	seedStatisticsImportTask(t, router.db, listingProductImportTask{TenantID: 101, StoreID: 1, Platform: "SHEIN", Region: "US", ProductID: "D3", Status: 5, CreateTime: timePtr(time.Date(2026, 5, 16, 10, 0, 0, 0, time.UTC))})

	req := httptest.NewRequest(http.MethodGet, "/store-statistics?date=2026-05-15", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /store-statistics = %d, body=%s", resp.Code, resp.Body.String())
	}
	var items []StoreStatistics
	if err := json.Unmarshal(resp.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %+v, want one store", items)
	}
	got := items[0]
	if got.CompletedCount != 1 || got.QueuedCount != 0 {
		t.Fatalf("statistics = %+v, want only tasks from 2026-05-15 counted", got)
	}
}

type storeStatisticsTestRouter struct {
	engine *gin.Engine
	db     *gorm.DB
}

func newStoreStatisticsTestRouter(t *testing.T) storeStatisticsTestRouter {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingStore{}, &listingProductImportTask{}); err != nil {
		t.Fatalf("migrate statistics tables: %v", err)
	}
	repo := NewGormStoreStatisticsRepository(db)
	handler := NewStoreStatisticsHandler(repo)
	engine := gin.New()
	engine.GET("/store-statistics", handler.ListStoreStatistics)
	return storeStatisticsTestRouter{engine: engine, db: db}
}

func seedStatisticsImportTask(t *testing.T, db *gorm.DB, task listingProductImportTask) listingProductImportTask {
	t.Helper()
	if task.CategoryID == 0 {
		task.CategoryID = 1
	}
	if err := db.Table("listing_product_import_task").Create(&task).Error; err != nil {
		t.Fatalf("seed import task: %v", err)
	}
	return task
}

func timePtr(value time.Time) *time.Time {
	return &value
}

package listingadmin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

type captureStoreStatisticsRepository struct {
	query StoreStatisticsQuery
	page  *StoreStatisticsPage
	err   error
}

func (r *captureStoreStatisticsRepository) ListStoreStatistics(_ context.Context, query StoreStatisticsQuery) (*StoreStatisticsPage, error) {
	r.query = query
	if r.err != nil {
		return nil, r.err
	}
	if r.page != nil {
		return r.page, nil
	}
	return &StoreStatisticsPage{Items: []StoreStatistics{}}, nil
}

func TestStoreStatisticsHandlerListsAutoListingStoresWithinTenant(t *testing.T) {
	router := newStoreStatisticsTestRouter(t)
	trueValue := true
	falseValue := false
	limit := 10
	seedStore(t, router.db, listingStore{
		ID:                1,
		TenantID:          101,
		OwnerUserID:       "user-101",
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
	var page StoreStatisticsPage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Total != 1 || page.Page != 1 || page.PageSize != 20 {
		t.Fatalf("page metadata = %+v, want total=1 page=1 pageSize=20", page)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %+v, want one auto listing store for tenant 101", page.Items)
	}
	got := page.Items[0]
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
	var page StoreStatisticsPage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Name != "Owned by A" {
		t.Fatalf("statistics items = %+v, want only user-a store", page.Items)
	}
}

func TestStoreStatisticsHandlerPlatformRoleStillUsesTenantAndOwnerScope(t *testing.T) {
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
	req.Header.Set("X-User-Roles", "platform_admin")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /store-statistics = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page StoreStatisticsPage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("statistics items = %+v, want both stores from tenant 101 only", page.Items)
	}
	for _, item := range page.Items {
		if item.TenantID != 101 {
			t.Fatalf("statistics item = %+v, want tenant 101", item)
		}
	}
}

func TestStoreStatisticsHandlerPlatformStatisticsUsesGlobalScope(t *testing.T) {
	repo := &captureStoreStatisticsRepository{page: &StoreStatisticsPage{Items: []StoreStatistics{}}}
	handler := NewStoreStatisticsHandler(repo)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/store-statistics", func(c *gin.Context) {
		MarkPlatformStoreAccess(c)
		handler.ListPlatformStoreStatistics(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/store-statistics", nil)
	req.Header.Set("X-Tenant-ID", "101")
	req.Header.Set("X-User-ID", "platform-admin")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /store-statistics = %d, body=%s", resp.Code, resp.Body.String())
	}
	if repo.query.TenantID != 0 || repo.query.OwnerUserID != "" {
		t.Fatalf("repo query = %+v, want global scope", repo.query)
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
	var page StoreStatisticsPage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Name != "Query Tenant Store" {
		t.Fatalf("statistics items = %+v, want query-tenant scoped store", page.Items)
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
	var page StoreStatisticsPage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items = %+v, want one store", page.Items)
	}
	got := page.Items[0]
	if got.CompletedCount != 1 || got.QueuedCount != 0 {
		t.Fatalf("statistics = %+v, want only tasks from 2026-05-15 counted", got)
	}
}

func TestStoreStatisticsHandlerSummaryJSONMatchesFrontendContract(t *testing.T) {
	router := newStoreStatisticsTestRouter(t)
	trueValue := true
	limitA := 5
	limitB := 7
	zeroLimit := 0
	seedStore(t, router.db, listingStore{
		ID:                1,
		TenantID:          101,
		Name:              "Tenant 101 Store A",
		Username:          "store-a",
		Password:          "secret",
		Platform:          "SHEIN",
		ShopType:          "semi",
		DailyLimit:        &limitA,
		DailyLimitType:    "fixed",
		EnableAutoListing: &trueValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})
	seedStore(t, router.db, listingStore{
		ID:                2,
		TenantID:          101,
		Name:              "Tenant 101 Store B",
		Username:          "store-b",
		Password:          "secret",
		Platform:          "TEMU",
		ShopType:          "semi",
		DailyLimit:        &limitB,
		DailyLimitType:    "fixed",
		EnableAutoListing: &trueValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})
	seedStore(t, router.db, listingStore{
		ID:                3,
		TenantID:          101,
		Name:              "Zero Limit Store",
		Username:          "store-c",
		Password:          "secret",
		Platform:          "AMAZON",
		ShopType:          "semi",
		DailyLimit:        &zeroLimit,
		DailyLimitType:    "fixed",
		EnableAutoListing: &trueValue,
		EnableAutoLogin:   &trueValue,
		Status:            0,
	})
	seedStatisticsImportTask(t, router.db, listingProductImportTask{TenantID: 101, StoreID: 1, ProductID: "P1", Status: 2, CreateTime: timePtr(time.Date(2026, 5, 15, 8, 0, 0, 0, time.UTC))})
	seedStatisticsImportTask(t, router.db, listingProductImportTask{TenantID: 101, StoreID: 2, ProductID: "P2", Status: 5, CreateTime: timePtr(time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC))})
	seedStatisticsImportTask(t, router.db, listingProductImportTask{TenantID: 101, StoreID: 3, ProductID: "P3", Status: 10, CreateTime: timePtr(time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC))})

	req := httptest.NewRequest(http.MethodGet, "/store-statistics?date=2026-05-15&page=1&page_size=1", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /store-statistics = %d, body=%s", resp.Code, resp.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	summaryValue, ok := payload["summary"]
	if !ok {
		t.Fatalf("response = %+v, want summary object", payload)
	}
	summary, ok := summaryValue.(map[string]any)
	if !ok {
		t.Fatalf("summary = %#v, want object", summaryValue)
	}
	for _, want := range []string{
		"completed_count",
		"daily_limit",
		"remaining_count",
		"queued_count",
		"hold_count",
	} {
		if _, ok := summary[want]; !ok {
			t.Fatalf("summary = %+v, want key %s", summary, want)
		}
	}
	if got := summary["completed_count"]; got != float64(1) {
		t.Fatalf("summary completed_count = %v, want 1", got)
	}
	if got := summary["daily_limit"]; got != float64(12) {
		t.Fatalf("summary daily_limit = %v, want 12 across all eligible stores", got)
	}
	if got := summary["remaining_count"]; got != float64(0) {
		t.Fatalf("summary remaining_count = %v, want 0", got)
	}
	if got := summary["queued_count"]; got != float64(1) {
		t.Fatalf("summary queued_count = %v, want 1", got)
	}
	if got := summary["hold_count"]; got != float64(1) {
		t.Fatalf("summary hold_count = %v, want 1", got)
	}
	for _, unwanted := range []string{"completedCount", "dailyLimit", "remainingCount", "queuedCount", "holdCount"} {
		if _, exists := summary[unwanted]; exists {
			t.Fatalf("summary = %+v, should not contain key %s", summary, unwanted)
		}
	}
}

func TestStoreStatisticsHandlerPassesPageParamsToRepository(t *testing.T) {
	repo := &captureStoreStatisticsRepository{page: &StoreStatisticsPage{Items: []StoreStatistics{}, Total: 0, Page: 2, PageSize: 1}}
	handler := NewStoreStatisticsHandler(repo)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/store-statistics", handler.ListStoreStatistics)

	req := httptest.NewRequest(http.MethodGet, "/store-statistics?page=2&page_size=1", nil)
	req.Header.Set("X-Tenant-ID", "101")
	req.Header.Set("X-User-ID", "user-101")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /store-statistics = %d, body=%s", resp.Code, resp.Body.String())
	}
	if repo.query.Page != 2 || repo.query.PageSize != 1 {
		t.Fatalf("repo query page/pageSize = %d/%d, want 2/1", repo.query.Page, repo.query.PageSize)
	}
}

func TestStoreStatisticsHandlerPlatformAccessClearsTenantAndOwnerScope(t *testing.T) {
	repo := &captureStoreStatisticsRepository{page: &StoreStatisticsPage{Items: []StoreStatistics{}}}
	handler := NewStoreStatisticsHandler(repo)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/store-statistics", func(c *gin.Context) {
		MarkPlatformStoreAccess(c)
		handler.ListPlatformStoreStatistics(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/store-statistics", nil)
	req.Header.Set("X-Tenant-ID", "101")
	req.Header.Set("X-User-ID", "platform-admin")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /store-statistics = %d, body=%s", resp.Code, resp.Body.String())
	}
	if repo.query.TenantID != 0 || repo.query.OwnerUserID != "" {
		t.Fatalf("repo query = %+v, want TenantID=0 and empty OwnerUserID", repo.query)
	}
}

func TestStoreStatisticsHandlerTenantScopePreservesTenantAndOwner(t *testing.T) {
	repo := &captureStoreStatisticsRepository{page: &StoreStatisticsPage{Items: []StoreStatistics{}}}
	handler := NewStoreStatisticsHandler(repo)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/store-statistics", handler.ListStoreStatistics)

	req := httptest.NewRequest(http.MethodGet, "/store-statistics", nil)
	req.Header.Set("X-Tenant-ID", "101")
	req.Header.Set("X-User-ID", "user-101")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /store-statistics = %d, body=%s", resp.Code, resp.Body.String())
	}
	if repo.query.TenantID != 101 || repo.query.OwnerUserID != "user-101" {
		t.Fatalf("repo query = %+v, want tenant-scoped query", repo.query)
	}
}

func TestStoreStatisticsHandlerRepositoryErrorReturnsInternalServerError(t *testing.T) {
	repo := &captureStoreStatisticsRepository{err: errors.New("boom")}
	handler := NewStoreStatisticsHandler(repo)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/store-statistics", handler.ListStoreStatistics)

	req := httptest.NewRequest(http.MethodGet, "/store-statistics", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("GET /store-statistics = %d, want 500", resp.Code)
	}
}

func TestStoreStatisticsHandlerRejectsInvalidDateFormat(t *testing.T) {
	router := newStoreStatisticsTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/store-statistics?date=2026/05/15", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("GET /store-statistics invalid date = %d, body=%s, want 400", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"invalid_date"`) {
		t.Fatalf("body = %s, want invalid_date", resp.Body.String())
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

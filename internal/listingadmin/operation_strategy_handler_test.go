package listingadmin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"
)

func TestOperationStrategyHandlerListsWithinRequestTenant(t *testing.T) {
	t.Parallel()

	router := newOperationStrategyTestRouter(t)
	seedOperationStrategy(t, router.db, listingOperationStrategy{
		TenantID:             101,
		StoreID:              11,
		Name:                 "SHEIN stock guard",
		Platform:             "SHEIN",
		Status:               0,
		StockChangeThreshold: 5,
		StockChangeAction:    "UPDATE_STOCK",
		OutOfStockAction:     "OFF_SHELF",
	})
	seedOperationStrategy(t, router.db, listingOperationStrategy{
		TenantID: 202,
		StoreID:  22,
		Name:     "Other tenant",
		Platform: "TEMU",
		Status:   0,
	})

	req := httptest.NewRequest(http.MethodGet, "/operation-strategies?page=1&page_size=20", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /operation-strategies = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page OperationStrategyPage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want one strategy", page)
	}
	if page.Items[0].Name != "SHEIN stock guard" || page.Items[0].TenantID != 101 {
		t.Fatalf("items = %+v, want tenant 101 strategy only", page.Items)
	}
}

func TestOperationStrategyHandlerCreatesWithRequestTenant(t *testing.T) {
	t.Parallel()

	router := newOperationStrategyTestRouter(t)
	body := bytes.NewBufferString(`{
		"storeId":11,
		"name":"SHEIN stock guard",
		"platform":"SHEIN",
		"status":0,
		"stockChangeThreshold":5,
		"stockChangeAction":"UPDATE_STOCK",
		"stockUpdateRatio":0.8,
		"outOfStockAction":"OFF_SHELF",
		"minProfitRate":20,
		"lowProfitAction":"UPDATE_PRICE",
		"priceUpdateMultiplier":1.2,
		"remark":"ok"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/operation-strategies", body)
	req.Header.Set("X-Tenant-ID", "303")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /operation-strategies = %d, body=%s", resp.Code, resp.Body.String())
	}
	var created OperationStrategy
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.ID == 0 || created.TenantID != 303 || created.StoreID != 11 || created.Platform != "SHEIN" {
		t.Fatalf("created = %+v, want tenant scoped strategy", created)
	}
}

func TestOperationStrategyHandlerSoftDeletesWithinTenant(t *testing.T) {
	t.Parallel()

	router := newOperationStrategyTestRouter(t)
	strategy := seedOperationStrategy(t, router.db, listingOperationStrategy{
		TenantID: 505,
		StoreID:  11,
		Name:     "SHEIN stock guard",
		Platform: "SHEIN",
		Status:   0,
	})

	req := httptest.NewRequest(http.MethodDelete, "/operation-strategies/1", nil)
	req.Header.Set("X-Tenant-ID", "505")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /operation-strategies/1 = %d, body=%s", resp.Code, resp.Body.String())
	}
	var row listingOperationStrategy
	if err := router.db.Table("listing_operation_strategy").Where("id = ?", strategy.ID).Take(&row).Error; err != nil {
		t.Fatalf("load deleted row: %v", err)
	}
	if row.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", row.Deleted)
	}
}

func newOperationStrategyTestRouter(t *testing.T) storeTestRouter {
	t.Helper()
	router := newStoreTestRouter(t)
	if err := router.db.AutoMigrate(&listingOperationStrategy{}); err != nil {
		t.Fatalf("migrate listing_operation_strategy: %v", err)
	}
	repo := NewGormOperationStrategyRepository(router.db)
	handler := NewOperationStrategyHandler(repo)
	router.engine.GET("/operation-strategies", handler.ListOperationStrategies)
	router.engine.POST("/operation-strategies", handler.CreateOperationStrategy)
	router.engine.DELETE("/operation-strategies/:id", handler.DeleteOperationStrategy)
	return router
}

func seedOperationStrategy(t *testing.T, db *gorm.DB, strategy listingOperationStrategy) listingOperationStrategy {
	t.Helper()
	if err := db.Table("listing_operation_strategy").Create(&strategy).Error; err != nil {
		t.Fatalf("seed operation strategy: %v", err)
	}
	return strategy
}

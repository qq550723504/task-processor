package listingadmin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestProductImportMappingHandlerListsMappingsWithinRequestTenant(t *testing.T) {
	t.Parallel()

	router := newProductImportMappingTestRouter(t)
	seedProductImportMapping(t, router.db, listingProductImportMapping{
		TenantID:                101,
		ImportTaskID:            1001,
		StoreID:                 11,
		Platform:                "SHEIN",
		Region:                  "US",
		ProductID:               "B001",
		SKU:                     "SKU-001",
		SalePriceMultiplier:     1.8,
		DiscountPriceMultiplier: 1.2,
		Status:                  1,
	})
	seedProductImportMapping(t, router.db, listingProductImportMapping{
		TenantID:     202,
		ImportTaskID: 1002,
		StoreID:      12,
		Platform:     "SHEIN",
		Region:       "US",
		ProductID:    "B002",
		Status:       1,
	})

	req := httptest.NewRequest(http.MethodGet, "/product-import-mappings?page=1&page_size=20&platform=SHEIN", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /product-import-mappings = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page ProductImportMappingPage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want one mapping", page)
	}
	if page.Items[0].ProductID != "B001" || page.Items[0].TenantID != 101 {
		t.Fatalf("items = %+v, want tenant 101 mapping only", page.Items)
	}
}

func TestProductImportMappingHandlerCreatesMappingWithRequestTenant(t *testing.T) {
	t.Parallel()

	router := newProductImportMappingTestRouter(t)
	body := bytes.NewBufferString(`{
		"importTaskId":1001,
		"storeId":11,
		"platform":"SHEIN",
		"region":"US",
		"productId":"B001",
		"parentProductId":"PARENT-1",
		"sku":"SKU-001",
		"platformProductId":"SPU-1",
		"filterRuleId":7,
		"filterRuleRange":"1-15",
		"profitRuleId":8,
		"salePriceMultiplier":1.8,
		"discountPriceMultiplier":1.2,
		"status":1,
		"remark":"ok"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/product-import-mappings", body)
	req.Header.Set("X-Tenant-ID", "303")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /product-import-mappings = %d, body=%s", resp.Code, resp.Body.String())
	}
	var created ProductImportMapping
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.ID == 0 || created.TenantID != 303 || created.ProductID != "B001" || created.SKU != "SKU-001" {
		t.Fatalf("created = %+v, want tenant scoped mapping", created)
	}
}

func TestProductImportMappingHandlerSoftDeletesWithinTenant(t *testing.T) {
	t.Parallel()

	router := newProductImportMappingTestRouter(t)
	mapping := seedProductImportMapping(t, router.db, listingProductImportMapping{
		TenantID:     505,
		ImportTaskID: 1001,
		StoreID:      11,
		Platform:     "SHEIN",
		Region:       "US",
		ProductID:    "B001",
		Status:       1,
	})

	req := httptest.NewRequest(http.MethodDelete, "/product-import-mappings/1", nil)
	req.Header.Set("X-Tenant-ID", "505")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /product-import-mappings/1 = %d, body=%s", resp.Code, resp.Body.String())
	}
	var row listingProductImportMapping
	if err := router.db.Table("listing_product_import_mapping").Where("id = ?", mapping.ID).Take(&row).Error; err != nil {
		t.Fatalf("load deleted row: %v", err)
	}
	if row.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", row.Deleted)
	}
}

func TestProductImportMappingHandlerRejectsInvalidNumericFilters(t *testing.T) {
	t.Parallel()

	router := newProductImportMappingTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/product-import-mappings?importTaskId=abc", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("GET /product-import-mappings?importTaskId=abc = %d, body=%s, want 400", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"invalid_import_task_id"`) {
		t.Fatalf("body = %s, want invalid_import_task_id", resp.Body.String())
	}
}

func newProductImportMappingTestRouter(t *testing.T) storeTestRouter {
	t.Helper()
	router := newStoreTestRouter(t)
	if err := router.db.AutoMigrate(&listingProductImportMapping{}); err != nil {
		t.Fatalf("migrate listing_product_import_mapping: %v", err)
	}
	repo := NewGormProductImportMappingRepository(router.db)
	handler := NewProductImportMappingHandler(repo)
	router.engine.GET("/product-import-mappings", handler.ListProductImportMappings)
	router.engine.POST("/product-import-mappings", handler.CreateProductImportMapping)
	router.engine.DELETE("/product-import-mappings/:id", handler.DeleteProductImportMapping)
	return router
}

func seedProductImportMapping(t *testing.T, db *gorm.DB, mapping listingProductImportMapping) listingProductImportMapping {
	t.Helper()
	if err := db.Table("listing_product_import_mapping").Create(&mapping).Error; err != nil {
		t.Fatalf("seed product import mapping: %v", err)
	}
	return mapping
}

package listingadmin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"
)

func TestProductDataHandlerListsDataWithinRequestTenant(t *testing.T) {
	t.Parallel()

	router := newProductDataTestRouter(t)
	seedProductData(t, router.db, listingProductData{
		TenantID:          101,
		StoreID:           11,
		Platform:          "SHEIN",
		Region:            "US",
		ProductID:         "B001",
		Title:             "Cotton shirt",
		OriginalPrice:     19.99,
		SpecialPrice:      15.99,
		PriceCurrency:     "USD",
		Stock:             "12",
		PlatformProductID: "SPU-001",
		ShelfStatus:       2,
		Status:            1,
	})
	seedProductData(t, router.db, listingProductData{
		TenantID:  202,
		StoreID:   12,
		Platform:  "SHEIN",
		Region:    "US",
		ProductID: "B002",
		Title:     "Other tenant",
		Status:    1,
	})

	req := httptest.NewRequest(http.MethodGet, "/product-data?page=1&page_size=20&platform=SHEIN", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /product-data = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page ProductDataPage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want one product", page)
	}
	if page.Items[0].ProductID != "B001" || page.Items[0].TenantID != 101 {
		t.Fatalf("items = %+v, want tenant 101 product only", page.Items)
	}
}

func TestProductDataHandlerCreatesDataWithRequestTenant(t *testing.T) {
	t.Parallel()

	router := newProductDataTestRouter(t)
	body := bytes.NewBufferString(`{
		"source":"sync",
		"importTaskId":1001,
		"storeId":11,
		"categoryId":5,
		"platform":"SHEIN",
		"region":"US",
		"parentProductId":"PARENT-1",
		"productId":"B001",
		"title":"Cotton shirt",
		"description":"Lightweight shirt",
		"originalPrice":19.99,
		"specialPrice":15.99,
		"priceCurrency":"USD",
		"stock":"12",
		"brand":"ACME",
		"category":"Apparel/Shirts",
		"mainImageUrl":"https://example.test/main.jpg",
		"imageUrls":["https://example.test/main.jpg"],
		"attributes":{"color":"white"},
		"sourceUrl":"https://example.test/p/B001",
		"status":1,
		"platformProductId":"SPU-001",
		"shelfStatus":2
	}`)
	req := httptest.NewRequest(http.MethodPost, "/product-data", body)
	req.Header.Set("X-Tenant-ID", "303")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /product-data = %d, body=%s", resp.Code, resp.Body.String())
	}
	var created ProductData
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.ID == 0 || created.TenantID != 303 || created.ProductID != "B001" || created.PlatformProductID != "SPU-001" {
		t.Fatalf("created = %+v, want tenant scoped product data", created)
	}
}

func TestProductDataHandlerSoftDeletesWithinTenant(t *testing.T) {
	t.Parallel()

	router := newProductDataTestRouter(t)
	product := seedProductData(t, router.db, listingProductData{
		TenantID:  505,
		StoreID:   11,
		Platform:  "SHEIN",
		Region:    "US",
		ProductID: "B001",
		Title:     "Cotton shirt",
		Status:    1,
	})

	req := httptest.NewRequest(http.MethodDelete, "/product-data/1", nil)
	req.Header.Set("X-Tenant-ID", "505")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /product-data/1 = %d, body=%s", resp.Code, resp.Body.String())
	}
	var row listingProductData
	if err := router.db.Table("listing_product_data").Where("id = ?", product.ID).Take(&row).Error; err != nil {
		t.Fatalf("load deleted row: %v", err)
	}
	if row.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", row.Deleted)
	}
}

func newProductDataTestRouter(t *testing.T) storeTestRouter {
	t.Helper()
	router := newStoreTestRouter(t)
	if err := router.db.AutoMigrate(&listingProductData{}); err != nil {
		t.Fatalf("migrate listing_product_data: %v", err)
	}
	repo := NewGormProductDataRepository(router.db)
	handler := NewProductDataHandler(repo)
	router.engine.GET("/product-data", handler.ListProductData)
	router.engine.POST("/product-data", handler.CreateProductData)
	router.engine.DELETE("/product-data/:id", handler.DeleteProductData)
	return router
}

func seedProductData(t *testing.T, db *gorm.DB, product listingProductData) listingProductData {
	t.Helper()
	if err := db.Table("listing_product_data").Create(&product).Error; err != nil {
		t.Fatalf("seed product data: %v", err)
	}
	return product
}

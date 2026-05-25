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

func TestCategoryHandlerListsCategoriesWithinRequestTenant(t *testing.T) {
	t.Parallel()

	router := newCategoryTestRouter(t)
	seedCategory(t, router.db, listingCategory{
		TenantID: 101,
		Name:     "Apparel",
		Code:     "APPAREL",
		ParentID: 0,
		Level:    1,
		Sort:     10,
		Status:   1,
	})
	seedCategory(t, router.db, listingCategory{
		TenantID: 202,
		Name:     "Other",
		Code:     "OTHER",
		ParentID: 0,
		Level:    1,
		Sort:     10,
		Status:   1,
	})

	req := httptest.NewRequest(http.MethodGet, "/categories?name=Apparel", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /categories = %d, body=%s", resp.Code, resp.Body.String())
	}
	var items []Category
	if err := json.Unmarshal(resp.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(items) != 1 || items[0].Code != "APPAREL" || items[0].TenantID != 101 {
		t.Fatalf("items = %+v, want tenant 101 category only", items)
	}
}

func TestCategoryHandlerRejectsInvalidNumericFilters(t *testing.T) {
	t.Parallel()

	router := newCategoryTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/categories?level=abc", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("GET /categories invalid filter = %d, body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"invalid_level"`) {
		t.Fatalf("response body = %s, want invalid_level", resp.Body.String())
	}
}

func TestCategoryHandlerCreatesCategoryWithRequestTenant(t *testing.T) {
	t.Parallel()

	router := newCategoryTestRouter(t)
	body := bytes.NewBufferString(`{
		"name":"Apparel",
		"code":"APPAREL",
		"parentId":0,
		"level":1,
		"sort":10,
		"icon":"shirt",
		"image":"/category/apparel.png",
		"description":"Wearable products",
		"status":1
	}`)
	req := httptest.NewRequest(http.MethodPost, "/categories", body)
	req.Header.Set("X-Tenant-ID", "303")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /categories = %d, body=%s", resp.Code, resp.Body.String())
	}
	var created Category
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.ID == 0 || created.TenantID != 303 || created.Code != "APPAREL" {
		t.Fatalf("created = %+v, want tenant scoped category", created)
	}
}

func TestCategoryHandlerRejectsDeleteWhenCategoryHasChildren(t *testing.T) {
	t.Parallel()

	router := newCategoryTestRouter(t)
	parent := seedCategory(t, router.db, listingCategory{
		TenantID: 505,
		Name:     "Apparel",
		Code:     "APPAREL",
		ParentID: 0,
		Level:    1,
		Sort:     10,
		Status:   1,
	})
	seedCategory(t, router.db, listingCategory{
		TenantID: 505,
		Name:     "Shirts",
		Code:     "SHIRTS",
		ParentID: parent.ID,
		Level:    2,
		Sort:     10,
		Status:   1,
	})

	req := httptest.NewRequest(http.MethodDelete, "/categories/1", nil)
	req.Header.Set("X-Tenant-ID", "505")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("DELETE /categories/1 = %d, body=%s, want 409", resp.Code, resp.Body.String())
	}
	var row listingCategory
	if err := router.db.Table("listing_category").Where("id = ?", parent.ID).Take(&row).Error; err != nil {
		t.Fatalf("load parent row: %v", err)
	}
	if row.Deleted != 0 {
		t.Fatalf("deleted = %d, want 0", row.Deleted)
	}
}

func newCategoryTestRouter(t *testing.T) storeTestRouter {
	t.Helper()
	router := newStoreTestRouter(t)
	if err := router.db.AutoMigrate(&listingCategory{}); err != nil {
		t.Fatalf("migrate listing_category: %v", err)
	}
	repo := NewGormCategoryRepository(router.db)
	handler := NewCategoryHandler(repo)
	router.engine.GET("/categories", handler.ListCategories)
	router.engine.POST("/categories", handler.CreateCategory)
	router.engine.DELETE("/categories/:id", handler.DeleteCategory)
	return router
}

func seedCategory(t *testing.T, db *gorm.DB, category listingCategory) listingCategory {
	t.Helper()
	if err := db.Table("listing_category").Create(&category).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}
	return category
}

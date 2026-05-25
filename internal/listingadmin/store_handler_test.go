package listingadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"

	"task-processor/internal/tenantbridge"
)

func TestStoreHandlerListsStoresWithinRequestTenant(t *testing.T) {
	router := newStoreTestRouter(t)
	seedStore(t, router.db, listingStore{
		TenantID: 101,
		Name:     "SHEIN US",
		Username: "shein-us",
		Password: "secret",
		Platform: "SHEIN",
		ShopType: "semi",
		Region:   "US",
		Status:   0,
	})
	seedStore(t, router.db, listingStore{
		TenantID: 202,
		Name:     "TEMU EU",
		Username: "temu-eu",
		Password: "secret",
		Platform: "TEMU",
		ShopType: "full",
		Region:   "DE",
		Status:   0,
	})

	req := httptest.NewRequest(http.MethodGet, "/stores?page=1&page_size=20", nil)
	req.Header.Set("X-Tenant-ID", "101")
	req.Header.Set("X-User-ID", "user-101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /stores = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page storePageResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Total != 1 {
		t.Fatalf("total = %d, want 1", page.Total)
	}
	if len(page.Items) != 1 || page.Items[0].Name != "SHEIN US" || page.Items[0].TenantID != 101 {
		t.Fatalf("items = %+v, want tenant 101 store only", page.Items)
	}
}

func TestStoreHandlerRejectsInvalidStatusFilter(t *testing.T) {
	router := newStoreTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/stores?status=abc", nil)
	req.Header.Set("X-Tenant-ID", "101")
	req.Header.Set("X-User-ID", "user-101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("GET /stores invalid filter = %d, body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"invalid_status"`) {
		t.Fatalf("body = %s, want invalid_status", resp.Body.String())
	}
}

func TestStoreHandlerRejectsInvalidBooleanFilter(t *testing.T) {
	router := newStoreTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/stores?enableAutoListing=maybe", nil)
	req.Header.Set("X-Tenant-ID", "101")
	req.Header.Set("X-User-ID", "user-101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("GET /stores invalid boolean filter = %d, body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"invalid_enable_auto_listing"`) {
		t.Fatalf("body = %s, want invalid_enable_auto_listing", resp.Body.String())
	}
}

func TestStoreHandlerCreatesStoreWithRequestTenant(t *testing.T) {
	router := newStoreTestRouter(t)
	body := bytes.NewBufferString(`{
		"storeId":"MANUAL-STORE-ID",
		"name":"SHEIN US",
		"username":"shein-us",
		"password":"secret",
		"platform":"SHEIN",
		"shopType":"semi",
		"region":"US",
		"dailyLimit":200,
		"enableAutoListing":true
	}`)
	req := httptest.NewRequest(http.MethodPost, "/stores", body)
	req.Header.Set("X-Tenant-ID", "303")
	req.Header.Set("X-User-ID", "user-303")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /stores = %d, body=%s", resp.Code, resp.Body.String())
	}
	var created Store
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.ID == 0 || created.TenantID != 303 || created.Name != "SHEIN US" {
		t.Fatalf("created = %+v, want tenant scoped store", created)
	}
	if created.StoreID != "" {
		t.Fatalf("created.StoreID = %q, want empty because storeId is auto populated after login", created.StoreID)
	}

	var row listingStore
	if err := router.db.Table("listing_store").Where("id = ?", created.ID).Take(&row).Error; err != nil {
		t.Fatalf("load created row: %v", err)
	}
	if row.StoreID != "" {
		t.Fatalf("row.StoreID = %q, want empty because client input storeId should be ignored", row.StoreID)
	}
	if row.TenantID != 303 || row.EnableAutoListing == nil || !*row.EnableAutoListing || row.OwnerUserID != "user-303" || row.CreatedBy != "user-303" || row.UpdatedBy != "user-303" {
		t.Fatalf("row = %+v, want tenant and boolean fields persisted", row)
	}
}

func TestStoreHandlerResolvesLegacyTenantIDFromMappedZitadelTenant(t *testing.T) {
	restore := tenantbridge.ConfigureLegacyTenantResolver(staticLegacyTenantResolver{
		mapping: map[string]int64{"373211199677923496": 227},
	})
	t.Cleanup(restore)

	router := newStoreTestRouter(t)
	seedStore(t, router.db, listingStore{
		TenantID: 227,
		Name:     "Mapped SHEIN",
		Username: "mapped-user",
		Password: "secret",
		Platform: "SHEIN",
		ShopType: "semi",
		Region:   "US",
		Status:   0,
	})

	req := httptest.NewRequest(http.MethodGet, "/stores?page=1&page_size=20", nil)
	req.Header.Set("X-Tenant-ID", "373211199677923496")
	req.Header.Set("X-User-ID", "user-227")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /stores = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page storePageResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].TenantID != 227 {
		t.Fatalf("items = %+v, want mapped legacy tenant 227 store", page.Items)
	}
}

func TestStoreHandlerOwnerScopeFiltersStoresByUser(t *testing.T) {
	t.Cleanup(SetOwnerScopeRequiredForTesting(true))

	router := newStoreTestRouter(t)
	seedStore(t, router.db, listingStore{
		TenantID:    101,
		OwnerUserID: "user-a",
		CreatedBy:   "user-a",
		UpdatedBy:   "user-a",
		Name:        "Owned by A",
		Username:    "owner-a",
		Password:    "secret",
		Platform:    "SHEIN",
		ShopType:    "semi",
		Region:      "US",
		Status:      0,
	})
	seedStore(t, router.db, listingStore{
		TenantID:    101,
		OwnerUserID: "user-b",
		CreatedBy:   "user-b",
		UpdatedBy:   "user-b",
		Name:        "Owned by B",
		Username:    "owner-b",
		Password:    "secret",
		Platform:    "SHEIN",
		ShopType:    "semi",
		Region:      "US",
		Status:      0,
	})

	req := httptest.NewRequest(http.MethodGet, "/stores?page=1&page_size=20", nil)
	req.Header.Set("X-Tenant-ID", "101")
	req.Header.Set("X-User-ID", "user-a")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /stores = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page storePageResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].OwnerUserID != "user-a" {
		t.Fatalf("owner-scoped items = %+v, want only user-a store", page.Items)
	}
}

func TestStoreHandlerPlatformAdminBypassesOwnerScope(t *testing.T) {
	t.Cleanup(SetOwnerScopeRequiredForTesting(true))

	router := newStoreTestRouter(t)
	seedStore(t, router.db, listingStore{
		TenantID:    101,
		OwnerUserID: "user-a",
		CreatedBy:   "user-a",
		UpdatedBy:   "user-a",
		Name:        "Owned by A",
		Username:    "owner-a",
		Password:    "secret",
		Platform:    "SHEIN",
		ShopType:    "semi",
		Region:      "US",
		Status:      0,
	})
	seedStore(t, router.db, listingStore{
		TenantID:    101,
		OwnerUserID: "user-b",
		CreatedBy:   "user-b",
		UpdatedBy:   "user-b",
		Name:        "Owned by B",
		Username:    "owner-b",
		Password:    "secret",
		Platform:    "SHEIN",
		ShopType:    "semi",
		Region:      "US",
		Status:      0,
	})

	req := httptest.NewRequest(http.MethodGet, "/stores?page=1&page_size=20", nil)
	req.Header.Set("X-Tenant-ID", "101")
	req.Header.Set("X-User-ID", "platform-admin")
	req.Header.Set("X-User-Roles", "platform_admin")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /stores = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page storePageResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("platform-admin items = %+v, want both stores", page.Items)
	}
}

func TestStoreHandlerSoftDeletesWithinTenant(t *testing.T) {
	router := newStoreTestRouter(t)
	store := seedStore(t, router.db, listingStore{
		TenantID: 404,
		Name:     "SHEIN US",
		Username: "shein-us",
		Password: "secret",
		Platform: "SHEIN",
		ShopType: "semi",
		Region:   "US",
		Status:   0,
	})

	req := httptest.NewRequest(http.MethodDelete, "/stores/1", nil)
	req.Header.Set("X-Tenant-ID", "404")
	req.Header.Set("X-User-ID", "user-404")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /stores/1 = %d, body=%s", resp.Code, resp.Body.String())
	}
	var row listingStore
	if err := router.db.Table("listing_store").Where("id = ?", store.ID).Take(&row).Error; err != nil {
		t.Fatalf("load deleted row: %v", err)
	}
	if row.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", row.Deleted)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/stores?page=1&page_size=20", nil)
	listReq.Header.Set("X-Tenant-ID", "404")
	listReq.Header.Set("X-User-ID", "user-404")
	listResp := httptest.NewRecorder()
	router.engine.ServeHTTP(listResp, listReq)

	var page storePageResponse
	if err := json.Unmarshal(listResp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("page = %+v, want deleted store hidden", page)
	}
}

func TestStoreHandlerListsDeletedStoresWithinTenant(t *testing.T) {
	router := newStoreTestRouter(t)
	seedStore(t, router.db, listingStore{
		TenantID: 505,
		Name:     "Deleted SHEIN",
		Username: "deleted",
		Password: "secret",
		Platform: "SHEIN",
		ShopType: "semi",
		Status:   0,
		Deleted:  1,
	})
	seedStore(t, router.db, listingStore{
		TenantID: 606,
		Name:     "Other Deleted",
		Username: "other",
		Password: "secret",
		Platform: "TEMU",
		ShopType: "semi",
		Status:   0,
		Deleted:  1,
	})

	req := httptest.NewRequest(http.MethodGet, "/stores/deleted", nil)
	req.Header.Set("X-Tenant-ID", "505")
	req.Header.Set("X-User-ID", "user-505")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /stores/deleted = %d, body=%s", resp.Code, resp.Body.String())
	}
	var stores []Store
	if err := json.Unmarshal(resp.Body.Bytes(), &stores); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(stores) != 1 || stores[0].Name != "Deleted SHEIN" || stores[0].TenantID != 505 {
		t.Fatalf("stores = %+v, want deleted tenant store only", stores)
	}
}

func TestStoreHandlerRestoresAndPermanentlyDeletesWithinTenant(t *testing.T) {
	router := newStoreTestRouter(t)
	store := seedStore(t, router.db, listingStore{
		TenantID: 707,
		Name:     "Deleted SHEIN",
		Username: "deleted",
		Password: "secret",
		Platform: "SHEIN",
		ShopType: "semi",
		Status:   0,
		Deleted:  1,
	})

	restoreReq := httptest.NewRequest(http.MethodPut, "/stores/1/restore", nil)
	restoreReq.Header.Set("X-Tenant-ID", "707")
	restoreReq.Header.Set("X-User-ID", "user-707")
	restoreResp := httptest.NewRecorder()
	router.engine.ServeHTTP(restoreResp, restoreReq)

	if restoreResp.Code != http.StatusOK {
		t.Fatalf("PUT /stores/1/restore = %d, body=%s", restoreResp.Code, restoreResp.Body.String())
	}
	var restored listingStore
	if err := router.db.Table("listing_store").Where("id = ?", store.ID).Take(&restored).Error; err != nil {
		t.Fatalf("load restored row: %v", err)
	}
	if restored.Deleted != 0 {
		t.Fatalf("deleted = %d, want 0 after restore", restored.Deleted)
	}

	if err := router.db.Table("listing_store").Where("id = ?", store.ID).Update("deleted", 1).Error; err != nil {
		t.Fatalf("mark deleted again: %v", err)
	}
	deleteReq := httptest.NewRequest(http.MethodDelete, "/stores/1/permanent", nil)
	deleteReq.Header.Set("X-Tenant-ID", "707")
	deleteReq.Header.Set("X-User-ID", "user-707")
	deleteResp := httptest.NewRecorder()
	router.engine.ServeHTTP(deleteResp, deleteReq)

	if deleteResp.Code != http.StatusOK {
		t.Fatalf("DELETE /stores/1/permanent = %d, body=%s", deleteResp.Code, deleteResp.Body.String())
	}
	var count int64
	if err := router.db.Table("listing_store").Where("id = ?", store.ID).Count(&count).Error; err != nil {
		t.Fatalf("count permanently deleted row: %v", err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want permanent delete", count)
	}
}

func TestStoreHandlerExtendsValidityFromExistingDate(t *testing.T) {
	router := newStoreTestRouter(t)
	validUntil := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	seedStore(t, router.db, listingStore{
		TenantID:   808,
		Name:       "SHEIN US",
		Username:   "shein",
		Password:   "secret",
		Platform:   "SHEIN",
		ShopType:   "semi",
		Status:     0,
		ValidUntil: &validUntil,
	})

	req := httptest.NewRequest(http.MethodPut, "/stores/1/extend-validity?days=30", nil)
	req.Header.Set("X-Tenant-ID", "808")
	req.Header.Set("X-User-ID", "user-808")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("PUT /stores/1/extend-validity = %d, body=%s", resp.Code, resp.Body.String())
	}
	var updated Store
	if err := json.Unmarshal(resp.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if updated.ValidUntil == nil || !updated.ValidUntil.Equal(validUntil.AddDate(0, 0, 30)) {
		t.Fatalf("validUntil = %v, want +30 days", updated.ValidUntil)
	}
}

func TestStoreHandlerRejectsInvalidValidityDays(t *testing.T) {
	router := newStoreTestRouter(t)
	validUntil := time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC)
	seedStore(t, router.db, listingStore{
		TenantID:   809,
		Name:       "SHEIN US",
		Username:   "shein",
		Password:   "secret",
		Platform:   "SHEIN",
		ShopType:   "semi",
		Status:     0,
		ValidUntil: &validUntil,
	})

	req := httptest.NewRequest(http.MethodPut, "/stores/1/extend-validity?days=0", nil)
	req.Header.Set("X-Tenant-ID", "809")
	req.Header.Set("X-User-ID", "user-809")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("PUT /stores/1/extend-validity?days=0 = %d, body=%s, want 400", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"invalid_days"`) {
		t.Fatalf("body = %s, want invalid_days", resp.Body.String())
	}
	var row listingStore
	if err := router.db.Table("listing_store").Where("id = ?", int64(1)).Take(&row).Error; err != nil {
		t.Fatalf("load row: %v", err)
	}
	if row.ValidUntil == nil || !row.ValidUntil.Equal(validUntil) {
		t.Fatalf("validUntil = %v, want unchanged %v", row.ValidUntil, validUntil)
	}
}

type storeTestRouter struct {
	engine *gin.Engine
	db     *gorm.DB
}

func newStoreTestRouter(t *testing.T) storeTestRouter {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Dialector{DriverName: "sqlite", DSN: ":memory:"}, &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&listingStore{}); err != nil {
		t.Fatalf("migrate listing_store: %v", err)
	}
	repo := NewGormStoreRepository(db)
	handler := NewStoreHandler(repo)
	engine := gin.New()
	engine.GET("/stores", handler.ListStores)
	engine.GET("/stores/deleted", handler.ListDeletedStores)
	engine.POST("/stores", handler.CreateStore)
	engine.DELETE("/stores/:id", handler.DeleteStore)
	engine.PUT("/stores/:id/restore", handler.RestoreStore)
	engine.DELETE("/stores/:id/permanent", handler.PermanentlyDeleteStore)
	engine.PUT("/stores/:id/extend-validity", handler.ExtendStoreValidity)
	return storeTestRouter{engine: engine, db: db}
}

func seedStore(t *testing.T, db *gorm.DB, store listingStore) listingStore {
	t.Helper()
	if store.OwnerUserID != "" && store.CreatedBy == "" {
		store.CreatedBy = store.OwnerUserID
	}
	if store.OwnerUserID != "" && store.UpdatedBy == "" {
		store.UpdatedBy = store.OwnerUserID
	}
	if err := db.Table("listing_store").Create(&store).Error; err != nil {
		t.Fatalf("seed store: %v", err)
	}
	return store
}

type staticLegacyTenantResolver struct {
	mapping map[string]int64
}

func (s staticLegacyTenantResolver) ResolveLegacyTenantID(_ context.Context, tenantID string) (int64, bool, error) {
	value, ok := s.mapping[tenantID]
	return value, ok, nil
}

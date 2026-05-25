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

func TestFilterRuleHandlerListsRulesWithinRequestTenant(t *testing.T) {
	t.Parallel()

	router := newFilterRuleTestRouter(t)
	seedFilterRule(t, router.db, listingFilterRule{
		TenantID:       101,
		Name:           "Amazon basic",
		RuleCode:       "FR-AMZ",
		StoreID:        11,
		PriceMin:       1,
		PriceMax:       99,
		StockMin:       10,
		RatingMin:      4.2,
		ReviewCountMin: 20,
		Status:         1,
	})
	seedFilterRule(t, router.db, listingFilterRule{
		TenantID: 202,
		Name:     "Other tenant",
		RuleCode: "FR-OTHER",
		Status:   1,
	})

	req := httptest.NewRequest(http.MethodGet, "/filter-rules?page=1&page_size=20", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /filter-rules = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page FilterRulePage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want one rule", page)
	}
	if page.Items[0].RuleCode != "FR-AMZ" || page.Items[0].TenantID != 101 {
		t.Fatalf("items = %+v, want tenant 101 rule only", page.Items)
	}
}

func TestFilterRuleHandlerRejectsInvalidNumericFilters(t *testing.T) {
	t.Parallel()

	router := newFilterRuleTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/filter-rules?storeId=abc", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("GET /filter-rules invalid filter = %d, body=%s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"error":"invalid_store_id"`) {
		t.Fatalf("response body = %s, want invalid_store_id", resp.Body.String())
	}
}

func TestFilterRuleHandlerCreatesRuleWithRequestTenant(t *testing.T) {
	t.Parallel()

	router := newFilterRuleTestRouter(t)
	body := bytes.NewBufferString(`{
		"name":"Amazon basic",
		"ruleCode":"FR-AMZ",
		"description":"基础筛选",
		"storeId":11,
		"categoryId":22,
		"priceType":"special",
		"priceMin":1.5,
		"priceMax":99.9,
		"stockMin":10,
		"ratingMin":4.2,
		"reviewCountMin":20,
		"deliveryTimeMax":7,
		"fulfillmentType":"FBA",
		"status":1,
		"remark":"ok"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/filter-rules", body)
	req.Header.Set("X-Tenant-ID", "303")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /filter-rules = %d, body=%s", resp.Code, resp.Body.String())
	}
	var created FilterRule
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.ID == 0 || created.TenantID != 303 || created.RuleCode != "FR-AMZ" || created.StoreID == nil || *created.StoreID != 11 {
		t.Fatalf("created = %+v, want tenant scoped rule", created)
	}
}

func TestFilterRuleHandlerUpdatesStatusWithinTenant(t *testing.T) {
	t.Parallel()

	router := newFilterRuleTestRouter(t)
	rule := seedFilterRule(t, router.db, listingFilterRule{
		TenantID: 404,
		Name:     "Amazon basic",
		RuleCode: "FR-AMZ",
		Status:   0,
	})

	body := bytes.NewBufferString(`{"status":1,"remark":"enabled"}`)
	req := httptest.NewRequest(http.MethodPatch, "/filter-rules/1/status", body)
	req.Header.Set("X-Tenant-ID", "404")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("PATCH /filter-rules/1/status = %d, body=%s", resp.Code, resp.Body.String())
	}
	var row listingFilterRule
	if err := router.db.Table("listing_filter_rule").Where("id = ?", rule.ID).Take(&row).Error; err != nil {
		t.Fatalf("load updated row: %v", err)
	}
	if row.Status != 1 || row.Remark != "enabled" {
		t.Fatalf("row = %+v, want status and remark updated", row)
	}
}

func TestFilterRuleHandlerSoftDeletesWithinTenant(t *testing.T) {
	t.Parallel()

	router := newFilterRuleTestRouter(t)
	rule := seedFilterRule(t, router.db, listingFilterRule{
		TenantID: 505,
		Name:     "Amazon basic",
		RuleCode: "FR-AMZ",
		Status:   1,
	})

	req := httptest.NewRequest(http.MethodDelete, "/filter-rules/1", nil)
	req.Header.Set("X-Tenant-ID", "505")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /filter-rules/1 = %d, body=%s", resp.Code, resp.Body.String())
	}
	var row listingFilterRule
	if err := router.db.Table("listing_filter_rule").Where("id = ?", rule.ID).Take(&row).Error; err != nil {
		t.Fatalf("load deleted row: %v", err)
	}
	if row.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", row.Deleted)
	}
}

func newFilterRuleTestRouter(t *testing.T) storeTestRouter {
	t.Helper()
	router := newStoreTestRouter(t)
	if err := router.db.AutoMigrate(&listingFilterRule{}); err != nil {
		t.Fatalf("migrate listing_filter_rule: %v", err)
	}
	repo := NewGormFilterRuleRepository(router.db)
	handler := NewFilterRuleHandler(repo)
	router.engine.GET("/filter-rules", handler.ListFilterRules)
	router.engine.POST("/filter-rules", handler.CreateFilterRule)
	router.engine.PATCH("/filter-rules/:id/status", handler.UpdateFilterRuleStatus)
	router.engine.DELETE("/filter-rules/:id", handler.DeleteFilterRule)
	return router
}

func seedFilterRule(t *testing.T, db *gorm.DB, rule listingFilterRule) listingFilterRule {
	t.Helper()
	if err := db.Table("listing_filter_rule").Create(&rule).Error; err != nil {
		t.Fatalf("seed filter rule: %v", err)
	}
	return rule
}

package listingadmin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"
)

func TestPricingRuleHandlerListsRulesWithinRequestTenant(t *testing.T) {
	t.Parallel()

	router := newPricingRuleTestRouter(t)
	seedPricingRule(t, router.db, listingPricingRule{
		TenantID:   101,
		Name:       "SHEIN auto price",
		RuleCode:   "AR-SHEIN",
		StoreID:    11,
		PriceMin:   1,
		PriceMax:   99,
		RuleType:   "multiple_fixed",
		RuleValue:  1.8,
		FixedValue: 2.5,
		Status:     1,
	})
	seedPricingRule(t, router.db, listingPricingRule{
		TenantID:  202,
		Name:      "Other tenant",
		RuleCode:  "AR-OTHER",
		RuleType:  "fixed",
		RuleValue: 1,
		Status:    1,
	})

	req := httptest.NewRequest(http.MethodGet, "/pricing-rules?page=1&page_size=20", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /pricing-rules = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page PricingRulePage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want one rule", page)
	}
	if page.Items[0].RuleCode != "AR-SHEIN" || page.Items[0].TenantID != 101 {
		t.Fatalf("items = %+v, want tenant 101 rule only", page.Items)
	}
}

func TestPricingRuleHandlerCreatesRuleWithRequestTenant(t *testing.T) {
	t.Parallel()

	router := newPricingRuleTestRouter(t)
	body := bytes.NewBufferString(`{
		"name":"SHEIN auto price",
		"ruleCode":"AR-SHEIN",
		"description":"自动核价",
		"storeId":11,
		"categoryId":22,
		"priceMin":1,
		"priceMax":99,
		"ruleType":"multiple_fixed",
		"ruleValue":1.8,
		"fixedValue":2.5,
		"acceptCondition":"price <= 99",
		"rejectCondition":"price > 99",
		"status":1,
		"remark":"ok"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/pricing-rules", body)
	req.Header.Set("X-Tenant-ID", "303")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /pricing-rules = %d, body=%s", resp.Code, resp.Body.String())
	}
	var created PricingRule
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.ID == 0 || created.TenantID != 303 || created.RuleCode != "AR-SHEIN" || created.StoreID == nil || *created.StoreID != 11 {
		t.Fatalf("created = %+v, want tenant scoped rule", created)
	}
}

func TestPricingRuleHandlerSoftDeletesWithinTenant(t *testing.T) {
	t.Parallel()

	router := newPricingRuleTestRouter(t)
	rule := seedPricingRule(t, router.db, listingPricingRule{
		TenantID:  505,
		Name:      "SHEIN auto price",
		RuleCode:  "AR-SHEIN",
		RuleType:  "fixed",
		RuleValue: 3,
		Status:    1,
	})

	req := httptest.NewRequest(http.MethodDelete, "/pricing-rules/1", nil)
	req.Header.Set("X-Tenant-ID", "505")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /pricing-rules/1 = %d, body=%s", resp.Code, resp.Body.String())
	}
	var row listingPricingRule
	if err := router.db.Table("listing_pricing_rule").Where("id = ?", rule.ID).Take(&row).Error; err != nil {
		t.Fatalf("load deleted row: %v", err)
	}
	if row.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", row.Deleted)
	}
}

func newPricingRuleTestRouter(t *testing.T) storeTestRouter {
	t.Helper()
	router := newStoreTestRouter(t)
	if err := router.db.AutoMigrate(&listingPricingRule{}); err != nil {
		t.Fatalf("migrate listing_pricing_rule: %v", err)
	}
	repo := NewGormPricingRuleRepository(router.db)
	handler := NewPricingRuleHandler(repo)
	router.engine.GET("/pricing-rules", handler.ListPricingRules)
	router.engine.POST("/pricing-rules", handler.CreatePricingRule)
	router.engine.DELETE("/pricing-rules/:id", handler.DeletePricingRule)
	return router
}

func seedPricingRule(t *testing.T, db *gorm.DB, rule listingPricingRule) listingPricingRule {
	t.Helper()
	if err := db.Table("listing_pricing_rule").Create(&rule).Error; err != nil {
		t.Fatalf("seed pricing rule: %v", err)
	}
	return rule
}

package listingadmin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gorm.io/gorm"
)

func TestProfitRuleHandlerListsRulesWithinRequestTenant(t *testing.T) {
	t.Parallel()

	router := newProfitRuleTestRouter(t)
	seedProfitRule(t, router.db, listingProfitRule{
		TenantID:                101,
		Name:                    "SHEIN margin",
		RuleCode:                "PR-SHEIN",
		StoreID:                 11,
		SalePriceMultiplier:     3,
		DiscountPriceMultiplier: 2.5,
		Status:                  1,
	})
	seedProfitRule(t, router.db, listingProfitRule{
		TenantID: 202,
		Name:     "Other tenant",
		RuleCode: "PR-OTHER",
		Status:   1,
	})

	req := httptest.NewRequest(http.MethodGet, "/profit-rules?page=1&page_size=20", nil)
	req.Header.Set("X-Tenant-ID", "101")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("GET /profit-rules = %d, body=%s", resp.Code, resp.Body.String())
	}
	var page ProfitRulePage
	if err := json.Unmarshal(resp.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v, want one rule", page)
	}
	if page.Items[0].RuleCode != "PR-SHEIN" || page.Items[0].TenantID != 101 {
		t.Fatalf("items = %+v, want tenant 101 rule only", page.Items)
	}
}

func TestProfitRuleHandlerCreatesRuleWithRequestTenant(t *testing.T) {
	t.Parallel()

	router := newProfitRuleTestRouter(t)
	body := bytes.NewBufferString(`{
		"name":"SHEIN margin",
		"ruleCode":"PR-SHEIN",
		"description":"基础利润",
		"storeId":11,
		"categoryId":22,
		"salePriceMultiplier":3,
		"discountPriceMultiplier":2.5,
		"status":1,
		"remark":"ok"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/profit-rules", body)
	req.Header.Set("X-Tenant-ID", "303")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("POST /profit-rules = %d, body=%s", resp.Code, resp.Body.String())
	}
	var created ProfitRule
	if err := json.Unmarshal(resp.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.ID == 0 || created.TenantID != 303 || created.RuleCode != "PR-SHEIN" || created.StoreID == nil || *created.StoreID != 11 {
		t.Fatalf("created = %+v, want tenant scoped rule", created)
	}
}

func TestProfitRuleHandlerSoftDeletesWithinTenant(t *testing.T) {
	t.Parallel()

	router := newProfitRuleTestRouter(t)
	rule := seedProfitRule(t, router.db, listingProfitRule{
		TenantID: 505,
		Name:     "SHEIN margin",
		RuleCode: "PR-SHEIN",
		Status:   1,
	})

	req := httptest.NewRequest(http.MethodDelete, "/profit-rules/1", nil)
	req.Header.Set("X-Tenant-ID", "505")
	resp := httptest.NewRecorder()
	router.engine.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /profit-rules/1 = %d, body=%s", resp.Code, resp.Body.String())
	}
	var row listingProfitRule
	if err := router.db.Table("listing_profit_rule").Where("id = ?", rule.ID).Take(&row).Error; err != nil {
		t.Fatalf("load deleted row: %v", err)
	}
	if row.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", row.Deleted)
	}
}

func newProfitRuleTestRouter(t *testing.T) storeTestRouter {
	t.Helper()
	router := newStoreTestRouter(t)
	if err := router.db.AutoMigrate(&listingProfitRule{}); err != nil {
		t.Fatalf("migrate listing_profit_rule: %v", err)
	}
	repo := NewGormProfitRuleRepository(router.db)
	handler := NewProfitRuleHandler(repo)
	router.engine.GET("/profit-rules", handler.ListProfitRules)
	router.engine.POST("/profit-rules", handler.CreateProfitRule)
	router.engine.DELETE("/profit-rules/:id", handler.DeleteProfitRule)
	return router
}

func seedProfitRule(t *testing.T, db *gorm.DB, rule listingProfitRule) listingProfitRule {
	t.Helper()
	if err := db.Table("listing_profit_rule").Create(&rule).Error; err != nil {
		t.Fatalf("seed profit rule: %v", err)
	}
	return rule
}

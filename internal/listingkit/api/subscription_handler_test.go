package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingsubscription"
)

func TestPlatformSubscriptionRequiresPlatformRole(t *testing.T) {
	router := platformSubscriptionTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/platform/subscriptions/org-target", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusForbidden, resp.Body.String())
	}
}

func TestPlatformSubscriptionCanOpenModuleForTenant(t *testing.T) {
	router := platformSubscriptionTestRouter(t)

	body, err := json.Marshal(map[string]any{
		"status": "active",
		"limits": map[string]int{"design_jobs": 12},
	})
	if err != nil {
		t.Fatal(err)
	}
	putReq := httptest.NewRequest(http.MethodPut, "/platform/subscriptions/org-target/entitlements/studio", bytes.NewReader(body))
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("X-User-Roles", "platform_admin")
	putResp := httptest.NewRecorder()
	router.ServeHTTP(putResp, putReq)

	if putResp.Code != http.StatusOK {
		t.Fatalf("put status = %d, want %d; body=%s", putResp.Code, http.StatusOK, putResp.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/platform/subscriptions/org-target", nil)
	getReq.Header.Set("X-User-Roles", "platform_admin")
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)

	if getResp.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getResp.Code, http.StatusOK, getResp.Body.String())
	}
	var summary listingsubscription.Summary
	if err := json.Unmarshal(getResp.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	if summary.TenantID != "org-target" {
		t.Fatalf("tenant id = %q, want org-target", summary.TenantID)
	}
	var studio *listingsubscription.EntitlementView
	for i := range summary.Entitlements {
		if summary.Entitlements[i].Module.Code == listingsubscription.ModuleStudio {
			studio = &summary.Entitlements[i]
			break
		}
	}
	if studio == nil || !studio.Allowed || studio.Limits["design_jobs"] != 12 {
		t.Fatalf("studio entitlement = %#v", studio)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/platform/subscriptions", nil)
	listReq.Header.Set("X-User-Roles", "platform_admin")
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listResp.Code, http.StatusOK, listResp.Body.String())
	}
	var listBody struct {
		Items []listingsubscription.TenantOverview `json:"items"`
	}
	if err := json.Unmarshal(listResp.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode tenant list: %v", err)
	}
	if len(listBody.Items) != 1 || listBody.Items[0].TenantID != "org-target" || listBody.Items[0].ActiveCount != 1 {
		t.Fatalf("tenant list = %#v", listBody.Items)
	}
}

func platformSubscriptionTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	repo := listingsubscription.NewMemRepository()
	service, err := listingsubscription.NewService(repo)
	if err != nil {
		t.Fatalf("create subscription service: %v", err)
	}
	h, err := NewHandler(&stubGenerationTaskService{}, WithSubscriptionService(service))
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}
	router := gin.New()
	router.GET("/platform/subscriptions", h.ListPlatformTenantSubscriptions)
	router.GET("/platform/subscriptions/:tenant_id", h.GetPlatformTenantSubscription)
	router.PUT("/platform/subscriptions/:tenant_id/entitlements/:module_code", h.UpsertPlatformTenantSubscriptionEntitlement)
	return router
}

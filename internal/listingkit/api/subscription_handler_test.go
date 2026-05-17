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

func TestPlatformSubscriptionCanApplyPlanForTenant(t *testing.T) {
	router := platformSubscriptionTestRouter(t)

	listReq := httptest.NewRequest(http.MethodGet, "/platform/subscription-plans", nil)
	listReq.Header.Set("X-User-Roles", "platform_admin")
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list plans status = %d, want %d; body=%s", listResp.Code, http.StatusOK, listResp.Body.String())
	}
	var listBody struct {
		Items []listingsubscription.PlanBundle `json:"items"`
	}
	if err := json.Unmarshal(listResp.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode plans: %v", err)
	}
	if len(listBody.Items) == 0 {
		t.Fatal("plans response is empty")
	}

	body, err := json.Marshal(map[string]any{
		"plan_code": listingsubscription.PlanProfessional,
		"status":    listingsubscription.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	applyReq := httptest.NewRequest(http.MethodPut, "/platform/subscriptions/org-target/plan", bytes.NewReader(body))
	applyReq.Header.Set("Content-Type", "application/json")
	applyReq.Header.Set("X-User-Roles", "platform_admin")
	applyResp := httptest.NewRecorder()
	router.ServeHTTP(applyResp, applyReq)
	if applyResp.Code != http.StatusOK {
		t.Fatalf("apply plan status = %d, want %d; body=%s", applyResp.Code, http.StatusOK, applyResp.Body.String())
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
	if summary.Subscription == nil || summary.Subscription.PlanCode != listingsubscription.PlanProfessional {
		t.Fatalf("summary subscription = %#v", summary.Subscription)
	}
	if summary.CurrentPlan == nil || summary.CurrentPlan.Plan.Code != listingsubscription.PlanProfessional {
		t.Fatalf("summary current plan = %#v", summary.CurrentPlan)
	}
}

func TestPlatformSubscriptionCanManagePlans(t *testing.T) {
	router := platformSubscriptionTestRouter(t)

	createBody, err := json.Marshal(map[string]any{
		"code":        "growth",
		"name":        "增长版",
		"description": "面向增长期租户",
		"sort_order":  25,
		"active":      true,
		"modules": []map[string]any{
			{"module_code": listingsubscription.ModuleStudio, "limits": map[string]int{"design_jobs": 50}, "sort_order": 10},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	createReq := httptest.NewRequest(http.MethodPost, "/platform/subscription-plans", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-User-Roles", "platform_admin")
	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, createReq)
	if createResp.Code != http.StatusOK {
		t.Fatalf("create plan status = %d, want %d; body=%s", createResp.Code, http.StatusOK, createResp.Body.String())
	}

	moduleBody, err := json.Marshal(map[string]any{
		"limits":     map[string]int{"storage_bytes": 5368709120},
		"sort_order": 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	moduleReq := httptest.NewRequest(http.MethodPut, "/platform/subscription-plans/growth/modules/oss_storage", bytes.NewReader(moduleBody))
	moduleReq.Header.Set("Content-Type", "application/json")
	moduleReq.Header.Set("X-User-Roles", "platform_admin")
	moduleResp := httptest.NewRecorder()
	router.ServeHTTP(moduleResp, moduleReq)
	if moduleResp.Code != http.StatusOK {
		t.Fatalf("module status = %d, want %d; body=%s", moduleResp.Code, http.StatusOK, moduleResp.Body.String())
	}

	statusBody, err := json.Marshal(map[string]any{"active": false})
	if err != nil {
		t.Fatal(err)
	}
	statusReq := httptest.NewRequest(http.MethodPut, "/platform/subscription-plans/growth/status", bytes.NewReader(statusBody))
	statusReq.Header.Set("Content-Type", "application/json")
	statusReq.Header.Set("X-User-Roles", "platform_admin")
	statusResp := httptest.NewRecorder()
	router.ServeHTTP(statusResp, statusReq)
	if statusResp.Code != http.StatusOK {
		t.Fatalf("status update = %d, want %d; body=%s", statusResp.Code, http.StatusOK, statusResp.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/platform/subscription-plans/growth/modules/studio", nil)
	deleteReq.Header.Set("X-User-Roles", "platform_admin")
	deleteResp := httptest.NewRecorder()
	router.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete module status = %d, want %d; body=%s", deleteResp.Code, http.StatusOK, deleteResp.Body.String())
	}

	var bundle listingsubscription.PlanBundle
	if err := json.Unmarshal(deleteResp.Body.Bytes(), &bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	if bundle.Plan.Code != "growth" || bundle.Plan.Active {
		t.Fatalf("bundle plan = %#v", bundle.Plan)
	}
	if len(bundle.Modules) != 1 || bundle.Modules[0].ModuleCode != listingsubscription.ModuleOSSStorage {
		t.Fatalf("bundle modules = %#v", bundle.Modules)
	}
}

func TestPlatformSubscriptionPlanTenantsAndAuditLogs(t *testing.T) {
	router := platformSubscriptionTestRouter(t)

	applyBody, err := json.Marshal(map[string]any{
		"plan_code": listingsubscription.PlanProfessional,
		"status":    listingsubscription.StatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	applyReq := httptest.NewRequest(http.MethodPut, "/platform/subscriptions/org-alpha/plan", bytes.NewReader(applyBody))
	applyReq.Header.Set("Content-Type", "application/json")
	applyReq.Header.Set("X-User-Roles", "platform_admin")
	applyResp := httptest.NewRecorder()
	router.ServeHTTP(applyResp, applyReq)
	if applyResp.Code != http.StatusOK {
		t.Fatalf("apply status = %d, want %d; body=%s", applyResp.Code, http.StatusOK, applyResp.Body.String())
	}

	tenantsReq := httptest.NewRequest(http.MethodGet, "/platform/subscription-plans/professional/tenants", nil)
	tenantsReq.Header.Set("X-User-Roles", "platform_admin")
	tenantsResp := httptest.NewRecorder()
	router.ServeHTTP(tenantsResp, tenantsReq)
	if tenantsResp.Code != http.StatusOK {
		t.Fatalf("tenants status = %d, want %d; body=%s", tenantsResp.Code, http.StatusOK, tenantsResp.Body.String())
	}
	var tenantsBody struct {
		Items []listingsubscription.TenantSubscription `json:"items"`
	}
	if err := json.Unmarshal(tenantsResp.Body.Bytes(), &tenantsBody); err != nil {
		t.Fatalf("decode tenants: %v", err)
	}
	if len(tenantsBody.Items) != 1 || tenantsBody.Items[0].TenantID != "org-alpha" {
		t.Fatalf("tenants body = %#v", tenantsBody.Items)
	}

	auditReq := httptest.NewRequest(http.MethodGet, "/platform/subscription-plans/professional/audit-logs", nil)
	auditReq.Header.Set("X-User-Roles", "platform_admin")
	auditResp := httptest.NewRecorder()
	router.ServeHTTP(auditResp, auditReq)
	if auditResp.Code != http.StatusOK {
		t.Fatalf("audit status = %d, want %d; body=%s", auditResp.Code, http.StatusOK, auditResp.Body.String())
	}
	var auditBody struct {
		Items []listingsubscription.AuditLog `json:"items"`
	}
	if err := json.Unmarshal(auditResp.Body.Bytes(), &auditBody); err != nil {
		t.Fatalf("decode audit: %v", err)
	}
	if len(auditBody.Items) == 0 || auditBody.Items[0].Action != "plan_apply" {
		t.Fatalf("audit body = %#v", auditBody.Items)
	}
}

func TestPlatformSubscriptionCanUseConfiguredAdminUser(t *testing.T) {
	router := platformSubscriptionTestRouterWithOptions(t, WithPlatformSubscriptionAccess([]string{"admin-user"}, nil))

	req := httptest.NewRequest(http.MethodGet, "/platform/subscriptions/org-target", nil)
	req.Header.Set("X-User-ID", "admin-user")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
}

func platformSubscriptionTestRouter(t *testing.T) *gin.Engine {
	return platformSubscriptionTestRouterWithOptions(t)
}

func platformSubscriptionTestRouterWithOptions(t *testing.T, opts ...HandlerOption) *gin.Engine {
	t.Helper()
	repo := listingsubscription.NewMemRepository()
	service, err := listingsubscription.NewService(repo)
	if err != nil {
		t.Fatalf("create subscription service: %v", err)
	}
	baseOpts := []HandlerOption{WithSubscriptionService(service)}
	baseOpts = append(baseOpts, opts...)
	h, err := NewHandler(&stubGenerationTaskService{}, baseOpts...)
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}
	router := gin.New()
	router.GET("/platform/subscriptions", h.ListPlatformTenantSubscriptions)
	router.GET("/platform/subscription-plans", h.ListPlatformSubscriptionPlans)
	router.POST("/platform/subscription-plans", h.UpsertPlatformSubscriptionPlan)
	router.PUT("/platform/subscription-plans/:plan_code", h.UpsertPlatformSubscriptionPlan)
	router.PUT("/platform/subscription-plans/:plan_code/modules/:module_code", h.UpsertPlatformSubscriptionPlanModule)
	router.DELETE("/platform/subscription-plans/:plan_code/modules/:module_code", h.DeletePlatformSubscriptionPlanModule)
	router.PUT("/platform/subscription-plans/:plan_code/status", h.SetPlatformSubscriptionPlanStatus)
	router.GET("/platform/subscription-plans/:plan_code/tenants", h.ListPlatformSubscriptionPlanTenants)
	router.GET("/platform/subscription-plans/:plan_code/audit-logs", h.ListPlatformSubscriptionPlanAuditLogs)
	router.GET("/platform/subscriptions/:tenant_id", h.GetPlatformTenantSubscription)
	router.PUT("/platform/subscriptions/:tenant_id/plan", h.ApplyPlatformTenantSubscriptionPlan)
	router.PUT("/platform/subscriptions/:tenant_id/entitlements/:module_code", h.UpsertPlatformTenantSubscriptionEntitlement)
	return router
}

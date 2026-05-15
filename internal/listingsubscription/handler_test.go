package listingsubscription

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandlerReturnsSummaryAndUpdatesEntitlement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newTestService(t)
	handler := NewHandler(svc)
	router := gin.New()
	router.GET("/subscription/me", handler.GetCurrentSubscription)
	router.PUT("/admin/subscription/entitlements/:module_code", handler.UpsertEntitlement)

	req := httptest.NewRequest(http.MethodGet, "/subscription/me", nil)
	req.Header.Set("X-Tenant-ID", "org-286")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("summary status = %d body=%s", resp.Code, resp.Body.String())
	}
	var summary Summary
	if err := json.Unmarshal(resp.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.TenantID != "org-286" || len(summary.Entitlements) == 0 {
		t.Fatalf("summary = %+v, want tenant and entitlements", summary)
	}
	if summary.Entitlements[0].Allowed {
		t.Fatal("unconfigured module should not be allowed")
	}

	req = httptest.NewRequest(http.MethodPut, "/admin/subscription/entitlements/studio", strings.NewReader(`{"status":"active","limits":{"design_jobs":10}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Tenant-ID", "org-286")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("upsert status = %d body=%s", resp.Code, resp.Body.String())
	}

	result, err := svc.Check(t.Context(), "org-286", ModuleStudio)
	if err != nil || !result.Allowed {
		t.Fatalf("Check() = %+v, %v; want allowed", result, err)
	}
}

func TestHandlerRejectsInvalidModule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(newTestService(t))
	router := gin.New()
	router.PUT("/admin/subscription/entitlements/:module_code", handler.UpsertEntitlement)

	req := httptest.NewRequest(http.MethodPut, "/admin/subscription/entitlements/unknown", strings.NewReader(`{"status":"active"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 body=%s", resp.Code, resp.Body.String())
	}
}

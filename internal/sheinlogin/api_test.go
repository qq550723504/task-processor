package sheinlogin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandlerListAccountsRejectsMissingTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(newTestService(t, &stubAutomation{}))
	router := gin.New()
	router.GET("/api/v1/shein-login/accounts", handler.ListAccounts)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shein-login/accounts", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusUnauthorized)
	}
}

func TestHandlerListAccountsUsesLoginUserTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(newTestService(t, &stubAutomation{}))
	router := gin.New()
	router.GET("/api/v1/shein-login/accounts", handler.ListAccounts)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shein-login/accounts", nil)
	req.Header.Set("login-user", url.QueryEscape(`{"tenantId":1}`))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
	var payload struct {
		Success bool            `json:"success"`
		Data    []AccountStatus `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Success || len(payload.Data) != 1 || payload.Data[0].Account.TenantID != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestHandlerStatusRejectsCrossTenantStore(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(newTestService(t, &stubAutomation{}))
	router := gin.New()
	router.GET("/api/v1/shein-login/accounts/:store_id/status", handler.Status)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/shein-login/accounts/2/status", nil)
	req.Header.Set("tenant-id", "9")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/shein-login/accounts/2/status", nil)
	req.Header.Set("tenant-id", "8")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant status = %d, want %d", resp.Code, http.StatusNotFound)
	}
}

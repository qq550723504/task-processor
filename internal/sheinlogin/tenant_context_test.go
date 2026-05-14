package sheinlogin

import (
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestTenantIDPrefersTenantHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("tenant-id", "286")
	req.Header.Set("login-user", url.QueryEscape(`{"tenantId":999}`))
	c.Request = req

	tenantID, err := requestTenantID(c)
	if err != nil {
		t.Fatalf("requestTenantID() error = %v", err)
	}
	if tenantID != 286 {
		t.Fatalf("tenantID = %d, want 286", tenantID)
	}
}

func TestRequestTenantIDFallsBackToLoginUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("login-user", url.QueryEscape(`{"id":42,"tenantId":286}`))
	c.Request = req

	tenantID, err := requestTenantID(c)
	if err != nil {
		t.Fatalf("requestTenantID() error = %v", err)
	}
	if tenantID != 286 {
		t.Fatalf("tenantID = %d, want 286", tenantID)
	}
}

func TestRequestTenantIDRejectsMissingTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/", nil)

	if _, err := requestTenantID(c); err == nil {
		t.Fatal("expected missing tenant error")
	}
}

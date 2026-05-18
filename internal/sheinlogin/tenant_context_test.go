package sheinlogin

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/tenantbridge"
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

func TestRequestTenantIDResolvesMappedZitadelTenant(t *testing.T) {
	restore := tenantbridge.ConfigureLegacyTenantResolver(testLegacyTenantResolver{
		mapping: map[string]int64{"373211199677923496": 227},
	})
	t.Cleanup(restore)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Tenant-ID", "373211199677923496")
	c.Request = req

	tenantID, err := requestTenantID(c)
	if err != nil {
		t.Fatalf("requestTenantID() error = %v", err)
	}
	if tenantID != 227 {
		t.Fatalf("tenantID = %d, want 227", tenantID)
	}
}

type testLegacyTenantResolver struct {
	mapping map[string]int64
}

func (s testLegacyTenantResolver) ResolveLegacyTenantID(_ context.Context, tenantID string) (int64, bool, error) {
	value, ok := s.mapping[tenantID]
	return value, ok, nil
}

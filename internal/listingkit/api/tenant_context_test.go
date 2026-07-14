package api

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/listingkit"
)

func TestRequestContextUsesVerifiedIdentityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	req, err := http.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Tenant-ID", "org-286")
	req.Header.Set("X-User-ID", "user-42")
	c.Request = req

	if got := requestTenantID(c); got != "org-286" {
		t.Fatalf("tenant id = %q, want org-286", got)
	}
	if got := requestUserID(c); got != "user-42" {
		t.Fatalf("user id = %q, want user-42", got)
	}
}

func TestRequestContextIgnoresLegacyLoginUserHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	req, err := http.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("login-user", `{"id":42,"tenantId":286}`)
	c.Request = req

	if got := requestTenantID(c); got != "default" {
		t.Fatalf("tenant id = %q, want default", got)
	}
	if got := requestUserID(c); got != "" {
		t.Fatalf("user id = %q, want empty", got)
	}
}

func TestRequestContextPrefersAuthenticatedIdentityOverCallerInputs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	req, err := http.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks?tenant_id=tenant-b&user_id=user-b", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Tenant-ID", "tenant-b")
	req.Header.Set("X-User-ID", "user-b")
	req = req.WithContext(listingkit.WithAuthenticatedIdentity(req.Context(), listingkit.AuthenticatedIdentity{
		TenantID: "tenant-a",
		UserID:   "user-a",
		Roles:    []string{"listingkit_operator"},
	}))
	c.Request = req

	if got := requestTenantID(c, "tenant-b"); got != "tenant-a" {
		t.Fatalf("tenant id = %q, want authenticated tenant-a", got)
	}
	if got := requestUserID(c); got != "user-a" {
		t.Fatalf("user id = %q, want authenticated user-a", got)
	}
	if got := requestRoles(c); len(got) != 1 || got[0] != "listingkit_operator" {
		t.Fatalf("roles = %#v, want authenticated operator role", got)
	}
}

func TestRequestExplicitTenantIDRejectsMissingTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	req, err := http.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks", nil)
	if err != nil {
		t.Fatal(err)
	}
	c.Request = req

	if got, ok := requestExplicitTenantID(c); ok || got != "" {
		t.Fatalf("tenant id = %q ok=%v, want empty false", got, ok)
	}
}

func TestRequestExplicitTenantIDAcceptsExplicitDefaultTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	req, err := http.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Tenant-ID", "default")
	c.Request = req

	if got, ok := requestExplicitTenantID(c); !ok || got != "default" {
		t.Fatalf("tenant id = %q ok=%v, want default true", got, ok)
	}
}

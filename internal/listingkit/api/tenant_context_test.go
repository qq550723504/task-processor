package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/authidentity"
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
	req = req.WithContext(authidentity.WithAuthenticatedIdentity(req.Context(), authidentity.AuthenticatedIdentity{
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

func TestRequestTenantContextRejectsCallerFallbackForAuthenticatedIdentityWithoutEffectiveTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req, err := http.NewRequest(http.MethodGet, "/api/v1/listing-kits/tasks?tenant_id=query-forged", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Tenant-ID", "header-forged")
	req = req.WithContext(authidentity.WithAuthenticatedIdentity(req.Context(), authidentity.AuthenticatedIdentity{
		UserID: "verified-user",
	}))
	c.Request = req

	if got, ok := requestExplicitTenantID(c, "candidate-forged"); ok || got != "" {
		t.Fatalf("explicit tenant = %q ok=%v, want blank false", got, ok)
	}
	if got := requestTenantID(c, "candidate-forged"); got != "" {
		t.Fatalf("tenant = %q, want blank without candidate/header/query/default fallback", got)
	}
	if _, ok := requireExplicitRequestContext(c, "candidate-forged"); ok {
		t.Fatal("requireExplicitRequestContext accepted authenticated identity without effective tenant")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestDetachedHandlerContextPreservesSafeIdentityForCredentialResolution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	baseCtx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(baseCtx, http.MethodPost, "/api/v1/listing-kits/studio/batches/run", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer raw-token-must-not-be-copied")
	req = req.WithContext(authidentity.WithAuthenticatedIdentity(req.Context(), authidentity.AuthenticatedIdentity{
		TenantID: "tenant-a",
		UserID:   "user-a",
		Roles:    []string{"listingkit_operator"},
	}))
	c.Request = req

	detached := detachedRequestContext(c)
	cancel()
	select {
	case <-detached.Done():
		t.Fatal("detached handler context inherited request cancellation")
	default:
	}

	verified, ok := authidentity.AuthenticatedIdentityFromContext(detached)
	if !ok || verified.TenantID != "tenant-a" || verified.UserID != "user-a" {
		t.Fatalf("verified identity = %+v, %v", verified, ok)
	}
	requestIdentity := listingkit.RequestIdentityFromContext(detached)
	if requestIdentity.TenantID != "tenant-a" || requestIdentity.UserID != "user-a" {
		t.Fatalf("request identity = %+v", requestIdentity)
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

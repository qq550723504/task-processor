package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"task-processor/internal/aicapability"
	openaiclient "task-processor/internal/infra/clients/openai"
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

func TestDetachedHandlerContextPreservesSafeIdentityForCredentialResolution(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	baseCtx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(baseCtx, http.MethodPost, "/api/v1/listing-kits/studio/batches/run", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer raw-token-must-not-be-copied")
	req = req.WithContext(listingkit.WithAuthenticatedIdentity(req.Context(), listingkit.AuthenticatedIdentity{
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

	verified, ok := listingkit.AuthenticatedIdentityFromContext(detached)
	if !ok || verified.TenantID != "tenant-a" || verified.UserID != "user-a" {
		t.Fatalf("verified identity = %+v, %v", verified, ok)
	}
	requestIdentity := listingkit.RequestIdentityFromContext(detached)
	if requestIdentity.TenantID != "tenant-a" || requestIdentity.UserID != "user-a" {
		t.Fatalf("request identity = %+v", requestIdentity)
	}

	resolver := &handlerContextCredentialResolver{}
	router := &handlerContextRouter{resolver: resolver}
	recorder := &handlerContextInvocationRecorder{}
	adapter, err := listingkit.NewStudioAIImageCapabilityAdapter(listingkit.StudioAIImageCapabilityAdapterConfig{
		Legacy:   handlerContextImageGenerator{},
		Router:   router,
		Recorder: recorder,
		Mode:     aicapability.RoutingModeShadow,
	})
	if err != nil {
		t.Fatalf("NewStudioAIImageCapabilityAdapter() error = %v", err)
	}
	if _, err := adapter.GenerateImage(detached, &listingkit.AIImageGenerateRequest{Model: "nanobanana", Prompt: "private-prompt"}); err != nil {
		t.Fatalf("GenerateImage() error = %v", err)
	}
	if resolver.identity.TenantID != "tenant-a" || resolver.identity.UserID != "user-a" {
		t.Fatalf("credential identity = %+v", resolver.identity)
	}
	if router.request.TenantID != "tenant-a" || router.request.UserID != "user-a" {
		t.Fatalf("route request identity = %+v", router.request)
	}
	if recorder.record.TenantID != "tenant-a" || recorder.record.UserID != "user-a" {
		t.Fatalf("invocation record identity = %+v", recorder.record)
	}
	recordText := fmt.Sprintf("%+v", recorder.record)
	for _, secret := range []string{"raw-token-must-not-be-copied", "test-secret", "private-prompt"} {
		if strings.Contains(recordText, secret) {
			t.Fatalf("invocation record contains sensitive value %q", secret)
		}
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

type handlerContextCredentialResolver struct {
	identity openaiclient.Identity
}

func (r *handlerContextCredentialResolver) ResolveClientConfig(ctx context.Context, _ string, _ *openaiclient.ClientConfig) (*openaiclient.ResolvedClientConfig, error) {
	r.identity = openaiclient.IdentityFromContext(ctx)
	return &openaiclient.ResolvedClientConfig{Config: &openaiclient.ClientConfig{
		APIKey:  "test-secret",
		BaseURL: "https://provider.example.test/v1",
		Model:   "test-model",
	}}, nil
}

type handlerContextRouter struct {
	resolver *handlerContextCredentialResolver
	request  aicapability.RouteRequest
}

func (r *handlerContextRouter) Decide(ctx context.Context, request aicapability.RouteRequest) (aicapability.RouteDecision, error) {
	r.request = request
	if _, err := r.resolver.ResolveClientConfig(ctx, "image_nanobanana", nil); err != nil {
		return aicapability.RouteDecision{}, err
	}
	return aicapability.RouteDecision{
		Capability:          request.Capability,
		Operation:           request.Operation,
		ProviderID:          "grsai_async",
		ModelID:             "test-model",
		RoutingKey:          "nanobanana",
		CredentialReference: "image_nanobanana",
	}, nil
}

type handlerContextInvocationRecorder struct {
	record aicapability.InvocationRecord
}

func (r *handlerContextInvocationRecorder) RecordInvocation(_ context.Context, record aicapability.InvocationRecord) error {
	r.record = record
	return nil
}

type handlerContextImageGenerator struct{}

func (handlerContextImageGenerator) GenerateImage(context.Context, *listingkit.AIImageGenerateRequest) (*listingkit.AIImageResponse, error) {
	return &listingkit.AIImageResponse{}, nil
}

func (handlerContextImageGenerator) EditImage(context.Context, *listingkit.AIImageEditRequest) (*listingkit.AIImageResponse, error) {
	return &listingkit.AIImageResponse{}, nil
}

func (handlerContextImageGenerator) GetDefaultModel() string { return "nanobanana" }

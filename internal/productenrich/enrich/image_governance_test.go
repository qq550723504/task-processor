package enrich_test

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/aicapability"
	"task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"
	"task-processor/internal/shared/aiidentity"
)

func TestGovernedImageAnalyzerRoutesAndRecordsInvocation(t *testing.T) {
	recorder := &imageInvocationRecorder{}
	provider := &routedImageManager{response: `{"color":"red"}`}
	analyzer, err := productenrichenrich.NewGovernedImageAnalyzer(provider, productenrichenrich.GovernedImageAnalyzerConfig{
		Router: staticImageRouter{}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedImageAnalyzer: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	got, err := analyzer.AnalyzeImage(ctx, "https://example.test/product.jpg", "extract image attributes")
	if err != nil {
		t.Fatalf("AnalyzeImage: %v", err)
	}
	if got != `{"color":"red"}` {
		t.Fatalf("response = %q", got)
	}
	if provider.route.CredentialReference != "vision" || !provider.called {
		t.Fatalf("provider route/call = %+v/%v", provider.route, provider.called)
	}
	if recorder.record.Capability != aicapability.CapabilityProductEnrichVision || recorder.record.Outcome != aicapability.InvocationSucceeded {
		t.Fatalf("record = %+v", recorder.record)
	}
}

func TestGovernedImageAnalyzerRecordsRouteFailureWithoutProviderCall(t *testing.T) {
	recorder := &imageInvocationRecorder{}
	provider := &routedImageManager{}
	analyzer, err := productenrichenrich.NewGovernedImageAnalyzer(provider, productenrichenrich.GovernedImageAnalyzerConfig{
		Router: failingImageRouter{err: aicapability.NewError(aicapability.ErrorPolicyDenied, string(aicapability.OperationProductEnrichImageAnalyze), nil)}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedImageAnalyzer: %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	if _, err := analyzer.AnalyzeImage(ctx, "image.jpg", "prompt"); aicapability.CategoryOf(err) != aicapability.ErrorPolicyDenied {
		t.Fatalf("error category = %q", aicapability.CategoryOf(err))
	}
	if provider.called {
		t.Fatal("provider called after route failure")
	}
	if recorder.record.RouteErrorCategory != aicapability.ErrorPolicyDenied || recorder.record.Outcome != aicapability.InvocationFailed {
		t.Fatalf("record = %+v", recorder.record)
	}
}

func TestProductUnderstandingUsesGovernedImageAnalyzerWithoutLegacyClientLookup(t *testing.T) {
	provider := &routedImageManager{response: `{"color":"red","material":"cotton","scene":"studio","usage":"daily"}`}
	analyzer, err := productenrichenrich.NewGovernedImageAnalyzer(provider, productenrichenrich.GovernedImageAnalyzerConfig{
		Router: staticImageRouter{}, Recorder: &imageInvocationRecorder{},
	})
	if err != nil {
		t.Fatalf("NewGovernedImageAnalyzer: %v", err)
	}
	understanding, err := productenrichenrich.NewProductUnderstandingWithCapabilities(provider, nil, analyzer)
	if err != nil {
		t.Fatalf("NewProductUnderstandingWithCapabilities: %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	attr, err := understanding.AnalyzeImage(ctx, "https://example.test/product.jpg")
	if err != nil {
		t.Fatalf("AnalyzeImage: %v", err)
	}
	if attr.Color != "red" || attr.Material != "cotton" {
		t.Fatalf("attributes = %+v", attr)
	}
}

type staticImageRouter struct{}

func (staticImageRouter) Decide(context.Context, aicapability.RouteRequest) (aicapability.RouteDecision, error) {
	return aicapability.RouteDecision{
		Capability: aicapability.CapabilityProductEnrichVision, Operation: aicapability.OperationProductEnrichImageAnalyze,
		ProviderID: "openai", ModelID: "vision-model", RoutingKey: "productenrich-vision",
		CredentialReference: "vision", PolicyVersion: "policy-v1", ConfigurationVersion: "config-v1",
	}, nil
}

type failingImageRouter struct{ err error }

func (r failingImageRouter) Decide(context.Context, aicapability.RouteRequest) (aicapability.RouteDecision, error) {
	return aicapability.RouteDecision{}, r.err
}

type routedImageManager struct {
	response string
	route    productenrich.LLMClientRoute
	called   bool
}

func (m *routedImageManager) GetClient(string) (productenrich.LLMClient, error) {
	return nil, errors.New("legacy path not expected")
}
func (m *routedImageManager) GetDefaultClient() productenrich.LLMClient { return nil }
func (m *routedImageManager) GetClientWithRoute(_ context.Context, _ string, route productenrich.LLMClientRoute) (productenrich.LLMClient, error) {
	m.route = route
	return &routedImageClient{manager: m}, nil
}

type routedImageClient struct{ manager *routedImageManager }

func (c *routedImageClient) Generate(context.Context, string) (string, error) { return "", nil }
func (c *routedImageClient) AnalyzeImage(context.Context, string, string) (string, error) {
	c.manager.called = true
	return c.manager.response, nil
}

type imageInvocationRecorder struct{ record aicapability.InvocationRecord }

func (r *imageInvocationRecorder) RecordInvocation(_ context.Context, record aicapability.InvocationRecord) error {
	r.record = record
	return nil
}

var _ productenrich.RoutedLLMManager = (*routedImageManager)(nil)

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

func TestGovernedTextGeneratorRoutesAndRecordsInvocation(t *testing.T) {
	recorder := &textInvocationRecorder{}
	provider := &routedTextManager{response: `{"title":"Lamp"}`}
	generator, err := productenrichenrich.NewGovernedTextGenerator(provider, productenrichenrich.GovernedTextGeneratorConfig{
		Router:   staticTextRouter{},
		Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a", BusinessTaskID: "task-a", TraceID: "trace-a"})
	got, err := generator.Generate(ctx, "extract attributes")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != `{"title":"Lamp"}` {
		t.Fatalf("response = %q", got)
	}
	if provider.route.CredentialReference != "fast" || provider.route.ConfigurationVersion != "config-v1" {
		t.Fatalf("route = %+v", provider.route)
	}
	if recorder.record.TenantID != "tenant-a" || recorder.record.Operation != aicapability.OperationProductEnrichTextExtract || recorder.record.Outcome != aicapability.InvocationSucceeded {
		t.Fatalf("record = %+v", recorder.record)
	}
	if recorder.record.PromptHash == "" || recorder.record.OutputHash == "" {
		t.Fatalf("record hashes missing = %+v", recorder.record)
	}
}

func TestGovernedTextGeneratorRecordsRouteFailureWithoutProviderCall(t *testing.T) {
	recorder := &textInvocationRecorder{}
	provider := &routedTextManager{}
	generator, err := productenrichenrich.NewGovernedTextGenerator(provider, productenrichenrich.GovernedTextGeneratorConfig{
		Router:   failingTextRouter{err: aicapability.NewError(aicapability.ErrorPolicyDenied, string(aicapability.OperationProductEnrichTextExtract), nil)},
		Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	if _, err := generator.Generate(ctx, "extract attributes"); aicapability.CategoryOf(err) != aicapability.ErrorPolicyDenied {
		t.Fatalf("error category = %q, want policy_denied", aicapability.CategoryOf(err))
	}
	if provider.called {
		t.Fatal("provider called after route failure")
	}
	if recorder.record.Outcome != aicapability.InvocationFailed || recorder.record.RouteErrorCategory != aicapability.ErrorPolicyDenied {
		t.Fatalf("record = %+v", recorder.record)
	}
}

func TestGovernedTextGeneratorFallsBackToLegacyClientWhenTenantIsOutsideRollout(t *testing.T) {
	provider := &routedTextManager{legacyResponse: `{"title":"Legacy"}`}
	generator, err := productenrichenrich.NewGovernedTextGenerator(provider, productenrichenrich.GovernedTextGeneratorConfig{
		Router:   failingTextRouter{err: aicapability.NewError(aicapability.ErrorPolicyDenied, string(aicapability.OperationProductEnrichTextExtract), nil)},
		Recorder: &textInvocationRecorder{}, FallbackClient: "fast",
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-not-enabled", UserID: "user-a"})
	got, err := generator.Generate(ctx, "extract attributes")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != `{"title":"Legacy"}` || !provider.legacyCalled {
		t.Fatalf("legacy fallback response/call = %q/%v", got, provider.legacyCalled)
	}
}

func TestProductUnderstandingUsesGovernedTextGeneratorWithoutLegacyClientLookup(t *testing.T) {
	provider := &routedTextManager{response: `{"title":"Lamp","attributes":{"wattage":"10W"}}`}
	generator, err := productenrichenrich.NewGovernedTextGenerator(provider, productenrichenrich.GovernedTextGeneratorConfig{
		Router: staticTextRouter{}, Recorder: &textInvocationRecorder{},
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}
	understanding, err := productenrichenrich.NewProductUnderstandingWithTextGenerator(provider, generator)
	if err != nil {
		t.Fatalf("NewProductUnderstandingWithTextGenerator: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	attr, err := understanding.ExtractTextAttributes(ctx, "A 10W lamp")
	if err != nil {
		t.Fatalf("ExtractTextAttributes: %v", err)
	}
	if attr.Title != "Lamp" || attr.Attributes["wattage"] != "10W" {
		t.Fatalf("attributes = %+v", attr)
	}
	if !provider.called {
		t.Fatal("governed provider was not called")
	}
}

type staticTextRouter struct{}

func (staticTextRouter) Decide(context.Context, aicapability.RouteRequest) (aicapability.RouteDecision, error) {
	return aicapability.RouteDecision{
		Capability:           aicapability.CapabilityProductEnrichText,
		Operation:            aicapability.OperationProductEnrichTextExtract,
		ProviderID:           "openai",
		ModelID:              "text-model",
		RoutingKey:           "productenrich-text",
		CredentialReference:  "fast",
		PolicyVersion:        "policy-v1",
		ConfigurationVersion: "config-v1",
	}, nil
}

type failingTextRouter struct{ err error }

func (r failingTextRouter) Decide(context.Context, aicapability.RouteRequest) (aicapability.RouteDecision, error) {
	return aicapability.RouteDecision{}, r.err
}

type routedTextManager struct {
	response       string
	legacyResponse string
	route          productenrich.LLMClientRoute
	called         bool
	legacyCalled   bool
}

func (m *routedTextManager) GetClient(string) (productenrich.LLMClient, error) {
	if m.legacyResponse == "" {
		return nil, errors.New("legacy path not expected")
	}
	return &legacyTextClient{manager: m}, nil
}
func (m *routedTextManager) GetDefaultClient() productenrich.LLMClient { return nil }
func (m *routedTextManager) GetClientWithRoute(_ context.Context, _ string, route productenrich.LLMClientRoute) (productenrich.LLMClient, error) {
	m.route = route
	return &routedTextClient{manager: m}, nil
}

type routedTextClient struct{ manager *routedTextManager }

type legacyTextClient struct{ manager *routedTextManager }

func (c *routedTextClient) Generate(context.Context, string) (string, error) {
	c.manager.called = true
	return c.manager.response, nil
}
func (c *routedTextClient) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", nil
}
func (c *legacyTextClient) Generate(context.Context, string) (string, error) {
	c.manager.legacyCalled = true
	return c.manager.legacyResponse, nil
}
func (c *legacyTextClient) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", nil
}

type textInvocationRecorder struct{ record aicapability.InvocationRecord }

func (r *textInvocationRecorder) RecordInvocation(_ context.Context, record aicapability.InvocationRecord) error {
	r.record = record
	return nil
}

var _ productenrich.RoutedLLMManager = (*routedTextManager)(nil)

package enrich_test

import (
	"context"
	"errors"
	"strings"
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
		Planner: staticExecutionPlanner{plan: activeImageExecutionPlan()}, LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: recorder,
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
	if len(recorder.records) != 1 || recorder.records[0].Capability != aicapability.CapabilityProductEnrichVision || recorder.records[0].Outcome != aicapability.InvocationSucceeded {
		t.Fatalf("records = %+v", recorder.records)
	}
	if record := recorder.records[0]; record.PromptKey != "productenrich.understanding.analyze_image" || record.PromptVersion != "v1" || record.PromptScope != "product_enrich" {
		t.Fatalf("understanding prompt metadata = %+v", record)
	}
}

func TestGovernedVisionQualityRecordsQualityPromptIdentity(t *testing.T) {
	recorder := &imageInvocationRecorder{}
	manager := &routedImageManager{response: `{"score":90}`}
	analyzer, err := productenrichenrich.NewGovernedImageAnalyzer(manager, productenrichenrich.GovernedImageAnalyzerConfig{
		Planner: staticExecutionPlanner{plan: aicapability.ExecutionPlan{
			Mode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive,
			Decision: aicapability.RouteDecision{
				Capability: aicapability.CapabilityProductEnrichVision, Operation: aicapability.OperationProductEnrichVisionQualityScore,
				ProviderID: "openai", ModelID: "vision-model", RoutingKey: "vision", CredentialReference: "vision",
				PolicyVersion: "policy-v1", ConfigurationVersion: "config-v1",
			},
		}},
		LegacyRouteMetadata: staticLegacyRouteMetadataResolver{},
		Recorder:            recorder,
		Capability:          aicapability.CapabilityProductEnrichVision,
		Operation:           aicapability.OperationProductEnrichVisionQualityScore,
		RequiredFeature:     aicapability.FeatureVisionAnalyze,
		PromptKey:           "productenrich.llm_scorer.image_scoring",
		PromptVersion:       "v1",
		PromptScope:         "product_enrich",
	})
	if err != nil {
		t.Fatalf("NewGovernedImageAnalyzer: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	if _, err := analyzer.AnalyzeImage(ctx, "https://image", "score prompt"); err != nil {
		t.Fatalf("AnalyzeImage: %v", err)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("records = %d, want 1", len(recorder.records))
	}
	if record := recorder.records[0]; record.PromptKey != "productenrich.llm_scorer.image_scoring" || record.PromptVersion != "v1" || record.PromptScope != "product_enrich" {
		t.Fatalf("quality prompt metadata = %+v", record)
	}
}

func TestGovernedImagePreparedExecutionPreservesExplicitScoringPromptIdentity(t *testing.T) {
	recorder := &imageInvocationRecorder{}
	analyzer, err := productenrichenrich.NewGovernedImageAnalyzer(&routedImageManager{response: `{"score":90}`}, productenrichenrich.GovernedImageAnalyzerConfig{
		Planner: staticExecutionPlanner{plan: activeImageExecutionPlan()}, LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedImageAnalyzer: %v", err)
	}
	preparer, ok := analyzer.(productenrich.ImageExecutionPreparer)
	if !ok {
		t.Fatal("governed image analyzer must prepare scoring executions")
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	execution, err := preparer.PrepareImage(ctx, "https://image", "score prompt", productenrich.ScorePromptIdentity{
		PromptKey: "productenrich.llm_scorer.image_scoring", PromptVersion: "prompt-v17", PromptScope: "product_enrich",
	})
	if err != nil {
		t.Fatalf("PrepareImage: %v", err)
	}
	identity := execution.ScoreCacheIdentity("80", "raw-input-hash")
	if identity.PromptKey != "productenrich.llm_scorer.image_scoring" || identity.PromptVersion != "prompt-v17" || identity.PromptScope != "product_enrich" {
		t.Fatalf("cache prompt identity = %+v", identity)
	}
	if _, err := execution.Invoke(ctx, aicapability.CacheStatusMiss); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("records = %d, want 1", len(recorder.records))
	}
	if record := recorder.records[0]; record.PromptKey != "productenrich.llm_scorer.image_scoring" || record.PromptVersion != "prompt-v17" || record.PromptScope != "product_enrich" {
		t.Fatalf("record prompt metadata = %+v", record)
	}
}

func TestGovernedImageAnalyzerPolicyDeniedDoesNotCallLegacyProvider(t *testing.T) {
	recorder := &imageInvocationRecorder{}
	provider := &routedImageManager{response: "active", legacyResponse: "named", defaultResponse: "default"}
	analyzer, err := productenrichenrich.NewGovernedImageAnalyzer(provider, productenrichenrich.GovernedImageAnalyzerConfig{
		Planner:             staticExecutionPlanner{err: aicapability.NewError(aicapability.ErrorPolicyDenied, string(aicapability.OperationProductEnrichImageAnalyze), nil)},
		LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedImageAnalyzer: %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	if _, err := analyzer.AnalyzeImage(ctx, "image.jpg", "prompt"); aicapability.CategoryOf(err) != aicapability.ErrorPolicyDenied {
		t.Fatalf("error category = %q", aicapability.CategoryOf(err))
	}
	if provider.called || provider.legacyCalled || len(provider.namedLookups) != 0 || provider.defaultLookup {
		t.Fatalf("provider used after policy denial: %+v", provider)
	}
	if len(recorder.records) != 1 || recorder.records[0].RouteErrorCategory != aicapability.ErrorPolicyDenied || recorder.records[0].Outcome != aicapability.InvocationFailed {
		t.Fatalf("records = %+v", recorder.records)
	}
}

func TestGovernedImageLegacyUsesDefaultAndRecordsOneSuccess(t *testing.T) {
	provider := &routedImageManager{defaultResponse: `{"color":"legacy"}`}
	recorder := &imageInvocationRecorder{}
	analyzer, err := productenrichenrich.NewGovernedImageAnalyzer(provider, productenrichenrich.GovernedImageAnalyzerConfig{
		Planner: staticExecutionPlanner{plan: aicapability.ExecutionPlan{
			Mode: aicapability.RoutingModeLegacy, RouteOutcome: aicapability.RouteOutcomeLegacy,
			LegacyClients: []string{"vision", "default"},
		}},
		LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedImageAnalyzer: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-legacy", UserID: "user-a"})
	got, err := analyzer.AnalyzeImage(ctx, "image.jpg", "prompt")
	if err != nil {
		t.Fatalf("AnalyzeImage: %v", err)
	}
	if got != `{"color":"legacy"}` || !provider.legacyCalled || !provider.defaultLookup {
		t.Fatalf("legacy response/calls = %q/%v/%v", got, provider.legacyCalled, provider.defaultLookup)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("records = %d, want 1", len(recorder.records))
	}
	record := recorder.records[0]
	if record.RouteMode != aicapability.RoutingModeLegacy || record.RouteOutcome != aicapability.RouteOutcomeLegacy || record.Outcome != aicapability.InvocationSucceeded || record.FallbackIndex != 1 {
		t.Fatalf("record = %+v", record)
	}
}

func TestGovernedImageLegacyProviderFailureRecordsOneFailure(t *testing.T) {
	provider := &routedImageManager{callErr: errors.New("legacy image provider unavailable")}
	recorder := &imageInvocationRecorder{}
	analyzer, err := productenrichenrich.NewGovernedImageAnalyzer(provider, productenrichenrich.GovernedImageAnalyzerConfig{
		Planner: staticExecutionPlanner{plan: aicapability.ExecutionPlan{
			Mode: aicapability.RoutingModeLegacy, RouteOutcome: aicapability.RouteOutcomeLegacy, LegacyClients: []string{"vision"},
		}},
		LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedImageAnalyzer: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-legacy", UserID: "user-a"})
	_, err = analyzer.AnalyzeImage(ctx, "image.jpg", "prompt")
	if aicapability.CategoryOf(err) != aicapability.ErrorProviderUnavailable {
		t.Fatalf("error category = %q, want provider_unavailable", aicapability.CategoryOf(err))
	}
	if !provider.legacyCalled || len(recorder.records) != 1 {
		t.Fatalf("provider called/records = %v/%d", provider.legacyCalled, len(recorder.records))
	}
	record := recorder.records[0]
	if record.RouteMode != aicapability.RoutingModeLegacy || record.RouteOutcome != aicapability.RouteOutcomeLegacy || record.Outcome != aicapability.InvocationFailed || record.ErrorCategory != aicapability.ErrorProviderUnavailable {
		t.Fatalf("record = %+v", record)
	}
}

func TestGovernedImageUnavailableLegacyCandidatesRecordCategorizedLegacyFailureWithoutProviderUsage(t *testing.T) {
	provider := &routedImageManager{}
	recorder := &imageInvocationRecorder{}
	analyzer, err := productenrichenrich.NewGovernedImageAnalyzer(provider, productenrichenrich.GovernedImageAnalyzerConfig{
		Planner: staticExecutionPlanner{plan: aicapability.ExecutionPlan{
			Mode: aicapability.RoutingModeLegacy, RouteOutcome: aicapability.RouteOutcomeLegacy, LegacyClients: []string{"vision", "default"},
		}},
		LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedImageAnalyzer: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-legacy", UserID: "user-a"})
	_, err = analyzer.AnalyzeImage(ctx, "image.jpg", "prompt")
	if aicapability.CategoryOf(err) != aicapability.ErrorCredentialUnavailable {
		t.Fatalf("error category = %q, want credential_unavailable", aicapability.CategoryOf(err))
	}
	if provider.called || provider.legacyCalled || len(recorder.records) != 1 {
		t.Fatalf("provider calls/records = %v/%v/%d", provider.called, provider.legacyCalled, len(recorder.records))
	}
	record := recorder.records[0]
	if record.RouteMode != aicapability.RoutingModeLegacy || record.RouteOutcome != aicapability.RouteOutcomeLegacy || record.ErrorCategory != aicapability.ErrorCredentialUnavailable {
		t.Fatalf("record = %+v", record)
	}
}

func TestProductUnderstandingUsesGovernedImageAnalyzerWithoutLegacyClientLookup(t *testing.T) {
	provider := &routedImageManager{response: `{"color":"red","material":"cotton","scene":"studio","usage":"daily"}`}
	analyzer, err := productenrichenrich.NewGovernedImageAnalyzer(provider, productenrichenrich.GovernedImageAnalyzerConfig{
		Planner: staticExecutionPlanner{plan: activeImageExecutionPlan()}, LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: &imageInvocationRecorder{},
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
	if !provider.called || provider.legacyCalled {
		t.Fatalf("active/legacy provider calls = %v/%v", provider.called, provider.legacyCalled)
	}
}

func activeImageExecutionPlan() aicapability.ExecutionPlan {
	return aicapability.ExecutionPlan{
		Mode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive,
		Decision: aicapability.RouteDecision{
			Capability: aicapability.CapabilityProductEnrichVision, Operation: aicapability.OperationProductEnrichImageAnalyze,
			ProviderID: "openai", ModelID: "vision-model", RoutingKey: "productenrich-vision",
			CredentialReference: "vision", PolicyVersion: "policy-v1", ConfigurationVersion: "config-v1",
		},
	}
}

type routedImageManager struct {
	response        string
	legacyResponse  string
	defaultResponse string
	callErr         error
	route           productenrich.LLMClientRoute
	called          bool
	legacyCalled    bool
	namedLookups    []string
	defaultLookup   bool
}

func (m *routedImageManager) GetClient(clientName string) (productenrich.LLMClient, error) {
	m.namedLookups = append(m.namedLookups, clientName)
	if clientName == "default" || (m.legacyResponse == "" && m.callErr == nil) {
		return nil, errors.New("legacy client unavailable")
	}
	return &legacyImageClient{manager: m, response: m.legacyResponse}, nil
}

func (m *routedImageManager) GetDefaultClient() productenrich.LLMClient {
	m.defaultLookup = true
	if m.defaultResponse == "" && m.callErr == nil {
		return nil
	}
	return &legacyImageClient{manager: m, response: m.defaultResponse}
}

func (m *routedImageManager) GetClientWithRoute(_ context.Context, clientName string, route productenrich.LLMClientRoute) (productenrich.LLMClient, error) {
	m.route = route
	if strings.HasSuffix(route.ConfigurationVersion, "-config-v1") {
		m.namedLookups = append(m.namedLookups, clientName)
		if clientName == "default" {
			m.defaultLookup = true
			if m.defaultResponse == "" && m.callErr == nil {
				return nil, productenrich.ErrLLMClientUnavailable
			}
			return &legacyImageClient{manager: m, response: m.defaultResponse}, nil
		}
		if m.legacyResponse == "" && m.callErr == nil {
			return nil, productenrich.ErrLLMClientUnavailable
		}
		return &legacyImageClient{manager: m, response: m.legacyResponse}, nil
	}
	return &routedImageClient{manager: m}, nil
}

type routedImageClient struct{ manager *routedImageManager }

type legacyImageClient struct {
	manager  *routedImageManager
	response string
}

func (c *routedImageClient) Generate(context.Context, string) (string, error) { return "", nil }
func (c *routedImageClient) AnalyzeImage(context.Context, string, string) (string, error) {
	c.manager.called = true
	return c.manager.response, c.manager.callErr
}
func (c *legacyImageClient) Generate(context.Context, string) (string, error) { return "", nil }
func (c *legacyImageClient) AnalyzeImage(context.Context, string, string) (string, error) {
	c.manager.legacyCalled = true
	return c.response, c.manager.callErr
}

type imageInvocationRecorder struct {
	records []aicapability.InvocationRecord
}

func (r *imageInvocationRecorder) RecordInvocation(_ context.Context, record aicapability.InvocationRecord) error {
	r.records = append(r.records, record)
	return nil
}

var _ productenrich.RoutedLLMManager = (*routedImageManager)(nil)

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

func TestGovernedTextGeneratorRoutesAndRecordsInvocation(t *testing.T) {
	recorder := &textInvocationRecorder{}
	provider := &routedTextManager{response: `{"title":"Lamp"}`}
	generator, err := productenrichenrich.NewGovernedTextGenerator(provider, productenrichenrich.GovernedTextGeneratorConfig{
		Planner: staticExecutionPlanner{plan: activeTextExecutionPlan()}, LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: recorder,
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
	if len(recorder.records) != 1 {
		t.Fatalf("records = %d, want 1", len(recorder.records))
	}
	record := recorder.records[0]
	if record.TenantID != "tenant-a" || record.Operation != aicapability.OperationProductEnrichTextExtract || record.Outcome != aicapability.InvocationSucceeded {
		t.Fatalf("record = %+v", record)
	}
	if record.PromptHash == "" || record.OutputHash == "" || record.CacheStatus != aicapability.CacheStatusNotApplicable {
		t.Fatalf("record attribution missing = %+v", record)
	}
}

func TestGovernedTextActivePlanRejectsBlankConfigurationVersionBeforeProviderCall(t *testing.T) {
	plan := activeTextExecutionPlan()
	plan.Decision.ConfigurationVersion = ""
	provider := &routedTextManager{response: "active response"}
	generator, err := productenrichenrich.NewGovernedTextGenerator(provider, productenrichenrich.GovernedTextGeneratorConfig{
		Planner: staticExecutionPlanner{plan: plan}, LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: &textInvocationRecorder{},
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	if _, err := generator.Generate(ctx, "prompt"); aicapability.CategoryOf(err) != aicapability.ErrorCapabilityUnavailable {
		t.Fatalf("error category = %q, want capability_unavailable", aicapability.CategoryOf(err))
	}
	if provider.called {
		t.Fatal("provider called for active plan with blank configuration version")
	}
}

func TestGovernedTextGeneratorRecordsRouteFailureWithoutProviderCall(t *testing.T) {
	recorder := &textInvocationRecorder{}
	provider := &routedTextManager{response: "active", legacyResponse: "named", defaultResponse: "default"}
	generator, err := productenrichenrich.NewGovernedTextGenerator(provider, productenrichenrich.GovernedTextGeneratorConfig{
		Planner:             staticExecutionPlanner{err: aicapability.NewError(aicapability.ErrorPolicyDenied, string(aicapability.OperationProductEnrichTextExtract), nil)},
		LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	if _, err := generator.Generate(ctx, "extract attributes"); aicapability.CategoryOf(err) != aicapability.ErrorPolicyDenied {
		t.Fatalf("error category = %q, want policy_denied", aicapability.CategoryOf(err))
	}
	if provider.called || provider.legacyCalled || len(provider.namedLookups) != 0 || provider.defaultLookup {
		t.Fatalf("provider used after policy denial: %+v", provider)
	}
	if len(recorder.records) != 1 || recorder.records[0].Outcome != aicapability.InvocationFailed || recorder.records[0].RouteErrorCategory != aicapability.ErrorPolicyDenied {
		t.Fatalf("records = %+v", recorder.records)
	}
}

func TestGovernedTextPlannerFailureRetainsReturnedLegacyPlanMetadata(t *testing.T) {
	recorder := &textInvocationRecorder{}
	provider := &routedTextManager{}
	generator, err := productenrichenrich.NewGovernedTextGenerator(provider, productenrichenrich.GovernedTextGeneratorConfig{
		Planner: staticExecutionPlanner{
			plan: aicapability.ExecutionPlan{Mode: aicapability.RoutingModeLegacy, RouteOutcome: aicapability.RouteOutcomeLegacy},
			err:  errors.New("legacy execution plan requires a client candidate"),
		},
		LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-legacy", UserID: "user-a"})
	_, err = generator.Generate(ctx, "prompt")
	if aicapability.CategoryOf(err) != aicapability.ErrorCapabilityUnavailable {
		t.Fatalf("error category = %q, want capability_unavailable", aicapability.CategoryOf(err))
	}
	if provider.called || provider.legacyCalled || len(recorder.records) != 1 {
		t.Fatalf("provider calls/records = %v/%v/%d", provider.called, provider.legacyCalled, len(recorder.records))
	}
	record := recorder.records[0]
	if record.RouteMode != aicapability.RoutingModeLegacy || record.RouteOutcome != aicapability.RouteOutcomeLegacy || record.ErrorCategory != aicapability.ErrorCapabilityUnavailable {
		t.Fatalf("record = %+v", record)
	}
}

func TestGovernedTextLegacyUsesDefaultAndRecordsOneSuccess(t *testing.T) {
	provider := &routedTextManager{defaultResponse: `{"title":"Legacy"}`}
	recorder := &textInvocationRecorder{}
	generator, err := productenrichenrich.NewGovernedTextGenerator(provider, productenrichenrich.GovernedTextGeneratorConfig{
		Planner: staticExecutionPlanner{plan: aicapability.ExecutionPlan{
			Mode: aicapability.RoutingModeLegacy, RouteOutcome: aicapability.RouteOutcomeLegacy,
			LegacyClients: []string{"fast", "default"},
		}},
		LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-legacy", UserID: "user-a"})
	got, err := generator.Generate(ctx, "extract attributes")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != `{"title":"Legacy"}` || !provider.legacyCalled || !provider.defaultLookup {
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

func TestGovernedTextLegacyProviderFailureRecordsOneFailure(t *testing.T) {
	providerErr := errors.New("legacy provider unavailable")
	provider := &routedTextManager{callErr: providerErr}
	recorder := &textInvocationRecorder{}
	generator, err := productenrichenrich.NewGovernedTextGenerator(provider, productenrichenrich.GovernedTextGeneratorConfig{
		Planner: staticExecutionPlanner{plan: aicapability.ExecutionPlan{
			Mode: aicapability.RoutingModeLegacy, RouteOutcome: aicapability.RouteOutcomeLegacy, LegacyClients: []string{"fast"},
		}},
		LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-legacy", UserID: "user-a"})
	_, err = generator.Generate(ctx, "extract attributes")
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

func TestGovernedTextUnavailableLegacyCandidatesRecordCategorizedLegacyFailureWithoutProviderUsage(t *testing.T) {
	provider := &routedTextManager{}
	recorder := &textInvocationRecorder{}
	generator, err := productenrichenrich.NewGovernedTextGenerator(provider, productenrichenrich.GovernedTextGeneratorConfig{
		Planner: staticExecutionPlanner{plan: aicapability.ExecutionPlan{
			Mode: aicapability.RoutingModeLegacy, RouteOutcome: aicapability.RouteOutcomeLegacy, LegacyClients: []string{"fast", "default"},
		}},
		LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-legacy", UserID: "user-a"})
	_, err = generator.Generate(ctx, "extract attributes")
	if aicapability.CategoryOf(err) != aicapability.ErrorCredentialUnavailable {
		t.Fatalf("error category = %q, want credential_unavailable", aicapability.CategoryOf(err))
	}
	if provider.called || provider.legacyCalled {
		t.Fatalf("provider called: active=%v legacy=%v", provider.called, provider.legacyCalled)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("records = %d, want 1", len(recorder.records))
	}
	record := recorder.records[0]
	if record.RouteMode != aicapability.RoutingModeLegacy || record.RouteOutcome != aicapability.RouteOutcomeLegacy || record.ErrorCategory != aicapability.ErrorCredentialUnavailable {
		t.Fatalf("record = %+v", record)
	}
}

func TestGovernedTextUnavailableLegacyMetadataPreventsUnattributedProviderCall(t *testing.T) {
	provider := &routedTextManager{legacyResponse: "must not be returned"}
	recorder := &textInvocationRecorder{}
	generator, err := productenrichenrich.NewGovernedTextGenerator(provider, productenrichenrich.GovernedTextGeneratorConfig{
		Planner: staticExecutionPlanner{plan: aicapability.ExecutionPlan{
			Mode: aicapability.RoutingModeLegacy, RouteOutcome: aicapability.RouteOutcomeLegacy, LegacyClients: []string{"fast"},
		}},
		LegacyRouteMetadata: failingLegacyRouteMetadataResolver{}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}

	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-legacy", UserID: "user-a"})
	_, err = generator.Generate(ctx, "extract attributes")
	if aicapability.CategoryOf(err) != aicapability.ErrorCredentialUnavailable {
		t.Fatalf("error category = %q, want credential_unavailable", aicapability.CategoryOf(err))
	}
	if provider.legacyCalled {
		t.Fatal("provider called without attributable legacy metadata")
	}
	if len(recorder.records) != 1 || recorder.records[0].RouteMode != aicapability.RoutingModeLegacy || recorder.records[0].ErrorCategory != aicapability.ErrorCredentialUnavailable {
		t.Fatalf("records = %+v", recorder.records)
	}
	if recorder.records[0].PromptTokens != 0 || recorder.records[0].CompletionTokens != 0 || recorder.records[0].EstimatedCostMicros != 0 {
		t.Fatalf("rejected record contains provider usage = %+v", recorder.records[0])
	}
}

func TestProductUnderstandingUsesGovernedTextGeneratorWithoutLegacyClientLookup(t *testing.T) {
	provider := &routedTextManager{response: `{"title":"Lamp","attributes":{"wattage":"10W"}}`}
	generator, err := productenrichenrich.NewGovernedTextGenerator(provider, productenrichenrich.GovernedTextGeneratorConfig{
		Planner: staticExecutionPlanner{plan: activeTextExecutionPlan()}, LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: &textInvocationRecorder{},
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
	if !provider.called || provider.legacyCalled {
		t.Fatalf("active/legacy provider calls = %v/%v", provider.called, provider.legacyCalled)
	}
}

func activeTextExecutionPlan() aicapability.ExecutionPlan {
	return aicapability.ExecutionPlan{
		Mode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive,
		Decision: aicapability.RouteDecision{
			Capability: aicapability.CapabilityProductEnrichText, Operation: aicapability.OperationProductEnrichTextExtract,
			ProviderID: "openai", ModelID: "text-model", RoutingKey: "productenrich-text",
			CredentialReference: "fast", PolicyVersion: "policy-v1", ConfigurationVersion: "config-v1",
		},
	}
}

type staticExecutionPlanner struct {
	plan aicapability.ExecutionPlan
	err  error
}

func (p staticExecutionPlanner) Plan(context.Context, aicapability.RouteRequest) (aicapability.ExecutionPlan, error) {
	return p.plan, p.err
}

type staticLegacyRouteMetadataResolver struct{}

func (staticLegacyRouteMetadataResolver) ResolveLegacyRoute(_ context.Context, clientName string) (aicapability.RouteDecision, error) {
	return aicapability.RouteDecision{
		ProviderID: "openai", ModelID: "legacy-model", RoutingKey: clientName,
		CredentialReference: clientName, ConfigurationVersion: clientName + "-config-v1",
	}, nil
}

type failingLegacyRouteMetadataResolver struct{}

func (failingLegacyRouteMetadataResolver) ResolveLegacyRoute(context.Context, string) (aicapability.RouteDecision, error) {
	return aicapability.RouteDecision{}, errors.New("metadata unavailable")
}

type routedTextManager struct {
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

func (m *routedTextManager) GetClient(clientName string) (productenrich.LLMClient, error) {
	m.namedLookups = append(m.namedLookups, clientName)
	if clientName == "default" || (m.legacyResponse == "" && m.callErr == nil) {
		return nil, errors.New("legacy client unavailable")
	}
	return &legacyTextClient{manager: m, response: m.legacyResponse}, nil
}

func (m *routedTextManager) GetDefaultClient() productenrich.LLMClient {
	m.defaultLookup = true
	if m.defaultResponse == "" && m.callErr == nil {
		return nil
	}
	return &legacyTextClient{manager: m, response: m.defaultResponse}
}

func (m *routedTextManager) GetClientWithRoute(_ context.Context, clientName string, route productenrich.LLMClientRoute) (productenrich.LLMClient, error) {
	m.route = route
	if strings.HasSuffix(route.ConfigurationVersion, "-config-v1") {
		m.namedLookups = append(m.namedLookups, clientName)
		if clientName == "default" {
			m.defaultLookup = true
			if m.defaultResponse == "" && m.callErr == nil {
				return nil, productenrich.ErrLLMClientUnavailable
			}
			return &legacyTextClient{manager: m, response: m.defaultResponse}, nil
		}
		if m.legacyResponse == "" && m.callErr == nil {
			return nil, productenrich.ErrLLMClientUnavailable
		}
		return &legacyTextClient{manager: m, response: m.legacyResponse}, nil
	}
	return &routedTextClient{manager: m}, nil
}

type routedTextClient struct{ manager *routedTextManager }

type legacyTextClient struct {
	manager  *routedTextManager
	response string
}

func (c *routedTextClient) Generate(context.Context, string) (string, error) {
	c.manager.called = true
	return c.manager.response, c.manager.callErr
}
func (c *routedTextClient) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", nil
}
func (c *legacyTextClient) Generate(context.Context, string) (string, error) {
	c.manager.legacyCalled = true
	return c.response, c.manager.callErr
}
func (c *legacyTextClient) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", nil
}

type textInvocationRecorder struct {
	records []aicapability.InvocationRecord
}

func (r *textInvocationRecorder) RecordInvocation(_ context.Context, record aicapability.InvocationRecord) error {
	r.records = append(r.records, record)
	return nil
}

var _ productenrich.RoutedLLMManager = (*routedTextManager)(nil)

package enrich

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/aicapability"
	productenrich "task-processor/internal/productenrich"
	"task-processor/internal/shared/aiidentity"
)

func TestLegacyExecutionResolvesMetadataThenUsesOnlyBoundClient(t *testing.T) {
	recorder := &preparedExecutionRecorder{}
	manager := &boundLegacyManager{}
	generator, err := NewGovernedTextGenerator(manager, GovernedTextGeneratorConfig{
		Planner: legacyBoundPlanner{clients: []string{"fast", "default"}},
		LegacyRouteMetadata: boundLegacyMetadata{routes: map[string]aicapability.RouteDecision{
			"fast": {ProviderID: "openai", ModelID: "fast-model", RoutingKey: "fast", CredentialReference: "fast", ConfigurationVersion: "static:v1"},
		}},
		Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	prepared, err := generator.(interface {
		PrepareText(context.Context, string, productenrich.ScorePromptIdentity) (productenrich.GovernedScoreExecution, error)
	}).PrepareText(ctx, "prompt", productenrich.ScorePromptIdentity{PromptKey: "key", PromptVersion: "v1", PromptScope: "product_enrich"})
	if err != nil {
		t.Fatalf("PrepareText: %v", err)
	}
	cacheIdentity := prepared.ScoreCacheIdentity("80", "input-hash")
	if _, err := prepared.Invoke(ctx, aicapability.CacheStatusMiss); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if manager.unboundCalls != 0 || len(manager.routes) != 1 || manager.routes[0].CredentialReference != "fast" || manager.routes[0].ConfigurationVersion != "static:v1" {
		t.Fatalf("unbound/bound routes = %d/%+v", manager.unboundCalls, manager.routes)
	}
	if manager.providerCalls != 1 || len(recorder.records) != 1 {
		t.Fatalf("provider calls/records = %d/%d", manager.providerCalls, len(recorder.records))
	}
	record := recorder.records[0]
	if record.ConfigurationVersion != manager.routes[0].ConfigurationVersion || cacheIdentity.ConfigurationVersion != record.ConfigurationVersion || record.CredentialReference != manager.routes[0].CredentialReference {
		t.Fatalf("route/cache/ledger mismatch = %+v / %+v / %+v", manager.routes[0], cacheIdentity, record)
	}
}

func TestLegacyExecutionPreservesNamedToDefaultFallbackUnderBoundLookup(t *testing.T) {
	manager := &boundLegacyManager{unavailable: map[string]bool{"fast": true}}
	recorder := &preparedExecutionRecorder{}
	generator, err := NewGovernedTextGenerator(manager, GovernedTextGeneratorConfig{
		Planner: legacyBoundPlanner{clients: []string{"fast", "default"}},
		LegacyRouteMetadata: boundLegacyMetadata{routes: map[string]aicapability.RouteDecision{
			"fast":    {ProviderID: "openai", ModelID: "fast-model", RoutingKey: "fast", CredentialReference: "fast", ConfigurationVersion: "fast-v1"},
			"default": {ProviderID: "openai", ModelID: "default-model", RoutingKey: "default", CredentialReference: "default", ConfigurationVersion: "default-v1"},
		}},
		Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	if _, err := generator.Generate(ctx, "prompt"); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if manager.unboundCalls != 0 || len(manager.routes) != 2 || manager.routes[0].CredentialReference != "fast" || manager.routes[1].CredentialReference != "default" {
		t.Fatalf("unbound/bound routes = %d/%+v", manager.unboundCalls, manager.routes)
	}
	if len(recorder.records) != 1 || recorder.records[0].FallbackIndex != 1 || recorder.records[0].ConfigurationVersion != "default-v1" {
		t.Fatalf("record = %+v", recorder.records)
	}
}

func TestLegacyExecutionVersionMismatchFailsBeforeProviderAndRecordsOnce(t *testing.T) {
	manager := &boundLegacyManager{configurationChanged: true}
	recorder := &preparedExecutionRecorder{}
	generator, err := NewGovernedTextGenerator(manager, GovernedTextGeneratorConfig{
		Planner: legacyBoundPlanner{clients: []string{"default"}},
		LegacyRouteMetadata: boundLegacyMetadata{routes: map[string]aicapability.RouteDecision{
			"default": {ProviderID: "openai", ModelID: "model-v1", RoutingKey: "default", CredentialReference: "default", ConfigurationVersion: "db:v1"},
		}},
		Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	_, err = generator.Generate(ctx, "prompt")
	if aicapability.CategoryOf(err) != aicapability.ErrorCredentialUnavailable {
		t.Fatalf("error = %v, category = %q", err, aicapability.CategoryOf(err))
	}
	if manager.unboundCalls != 0 || manager.providerCalls != 0 || len(manager.routes) != 1 {
		t.Fatalf("unbound/provider/routes = %d/%d/%+v", manager.unboundCalls, manager.providerCalls, manager.routes)
	}
	if len(recorder.records) != 1 || recorder.records[0].Outcome != aicapability.InvocationFailed || recorder.records[0].ConfigurationVersion != "db:v1" {
		t.Fatalf("records = %+v", recorder.records)
	}
}

func TestLegacyExecutionContinuesOnlyUnavailableCandidatesThenFailsWithoutProvider(t *testing.T) {
	manager := &boundLegacyManager{unavailable: map[string]bool{"fast": true, "default": true}}
	recorder := &preparedExecutionRecorder{}
	generator, err := NewGovernedTextGenerator(manager, GovernedTextGeneratorConfig{
		Planner: legacyBoundPlanner{clients: []string{"fast", "default"}},
		LegacyRouteMetadata: boundLegacyMetadata{routes: map[string]aicapability.RouteDecision{
			"fast":    {ProviderID: "openai", ModelID: "fast-model", RoutingKey: "fast", CredentialReference: "fast", ConfigurationVersion: "fast-v1"},
			"default": {ProviderID: "openai", ModelID: "default-model", RoutingKey: "default", CredentialReference: "default", ConfigurationVersion: "default-v1"},
		}}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	_, err = generator.Generate(ctx, "prompt")
	if aicapability.CategoryOf(err) != aicapability.ErrorCredentialUnavailable || manager.providerCalls != 0 || len(manager.routes) != 2 {
		t.Fatalf("error/provider/routes = %v/%d/%+v", err, manager.providerCalls, manager.routes)
	}
	if len(recorder.records) != 1 || recorder.records[0].Outcome != aicapability.InvocationFailed || recorder.records[0].FallbackIndex != 1 {
		t.Fatalf("records = %+v", recorder.records)
	}
}

func TestLegacyExecutionRejectsNonRoutedManagerBeforeUnboundLookupAndRecordsOnce(t *testing.T) {
	manager := &nonRoutedLegacyManager{}
	recorder := &preparedExecutionRecorder{}
	generator, err := NewGovernedTextGenerator(manager, GovernedTextGeneratorConfig{
		Planner: legacyBoundPlanner{clients: []string{"default"}},
		LegacyRouteMetadata: boundLegacyMetadata{routes: map[string]aicapability.RouteDecision{
			"default": {ProviderID: "openai", ModelID: "model", RoutingKey: "default", CredentialReference: "default", ConfigurationVersion: "v1"},
		}}, Recorder: recorder,
	})
	if err != nil {
		t.Fatalf("NewGovernedTextGenerator: %v", err)
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	_, err = generator.Generate(ctx, "prompt")
	if aicapability.CategoryOf(err) != aicapability.ErrorCapabilityUnavailable || manager.unboundCalls != 0 {
		t.Fatalf("error/unbound calls = %v/%d", err, manager.unboundCalls)
	}
	if len(recorder.records) != 1 || recorder.records[0].Outcome != aicapability.InvocationFailed {
		t.Fatalf("records = %+v", recorder.records)
	}
}

type legacyBoundPlanner struct{ clients []string }

func (p legacyBoundPlanner) Plan(context.Context, aicapability.RouteRequest) (aicapability.ExecutionPlan, error) {
	return aicapability.ExecutionPlan{Mode: aicapability.RoutingModeLegacy, RouteOutcome: aicapability.RouteOutcomeLegacy, LegacyClients: p.clients}, nil
}

type boundLegacyMetadata struct {
	routes map[string]aicapability.RouteDecision
	errors map[string]error
}

func (m boundLegacyMetadata) ResolveLegacyRoute(_ context.Context, clientName string) (aicapability.RouteDecision, error) {
	return m.routes[clientName], m.errors[clientName]
}

type boundLegacyManager struct {
	unavailable          map[string]bool
	configurationChanged bool
	unboundCalls         int
	providerCalls        int
	routes               []productenrich.LLMClientRoute
}

func (m *boundLegacyManager) GetClient(string) (productenrich.LLMClient, error) {
	m.unboundCalls++
	return boundLegacyClient{manager: m}, nil
}

func (m *boundLegacyManager) GetDefaultClient() productenrich.LLMClient {
	m.unboundCalls++
	return boundLegacyClient{manager: m}
}

func (m *boundLegacyManager) GetClientWithRoute(_ context.Context, clientName string, route productenrich.LLMClientRoute) (productenrich.LLMClient, error) {
	m.routes = append(m.routes, route)
	if m.configurationChanged {
		return nil, productenrich.ErrLLMClientConfigurationChanged
	}
	if m.unavailable[clientName] {
		return nil, productenrich.ErrLLMClientUnavailable
	}
	return boundLegacyClient{manager: m}, nil
}

type boundLegacyClient struct{ manager *boundLegacyManager }

func (c boundLegacyClient) Generate(context.Context, string) (string, error) {
	c.manager.providerCalls++
	return `{"score":90}`, nil
}

func (c boundLegacyClient) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", errors.New("image call not expected")
}

type nonRoutedLegacyManager struct{ unboundCalls int }

func (m *nonRoutedLegacyManager) GetClient(string) (productenrich.LLMClient, error) {
	m.unboundCalls++
	return nil, productenrich.ErrLLMClientUnavailable
}

func (m *nonRoutedLegacyManager) GetDefaultClient() productenrich.LLMClient {
	m.unboundCalls++
	return nil
}

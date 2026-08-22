package enrich

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/aicapability"
	productenrich "task-processor/internal/productenrich"
	"task-processor/internal/shared/aiidentity"
)

func TestValidExecutionDecisionRejectsBlankConfigurationVersion(t *testing.T) {
	decision := aicapability.RouteDecision{
		Capability: aicapability.CapabilityProductEnrichText,
		Operation:  aicapability.OperationProductEnrichTextExtract,
		ProviderID: "openai", ModelID: "model", RoutingKey: "fast", CredentialReference: "fast",
	}
	if validExecutionDecision(decision, decision.Capability, decision.Operation) {
		t.Fatal("active decision with blank configuration version is executable")
	}
}

func TestPreparedExecutionInvokeRecordsRequestedCacheStatusExactlyOnce(t *testing.T) {
	recorder := &preparedExecutionRecorder{}
	execution := &preparedExecution{
		identity: aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"},
		plan: aicapability.ExecutionPlan{
			Mode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive,
		},
		decision: aicapability.RouteDecision{
			Capability: aicapability.CapabilityProductEnrichText, Operation: aicapability.OperationProductEnrichTextExtract,
			ProviderID: "openai", ModelID: "text-model", RoutingKey: "productenrich-text", CredentialReference: "fast",
		},
		promptKey: "prompt-key", promptVersion: "v1", promptScope: "product_enrich",
		prompt: "prompt", input: "input",
		call:     func(context.Context) (string, error) { return "response", nil },
		recorder: recorder,
	}

	got, err := execution.invoke(context.Background(), aicapability.CacheStatusMiss)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if got != "response" {
		t.Fatalf("response = %q", got)
	}
	if len(recorder.records) != 1 {
		t.Fatalf("records = %d, want 1", len(recorder.records))
	}
	if recorder.records[0].CacheStatus != aicapability.CacheStatusMiss || recorder.records[0].Outcome != aicapability.InvocationSucceeded {
		t.Fatalf("record = %+v", recorder.records[0])
	}
}

func TestPreparedExecutionRecorderFailureKeepsProviderResultAndCallsCallback(t *testing.T) {
	recordErr := errors.New("ledger unavailable")
	recorder := &preparedExecutionRecorder{err: recordErr}
	callbackCalls := 0
	execution := &preparedExecution{
		identity: aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"},
		plan:     aicapability.ExecutionPlan{Mode: aicapability.RoutingModeLegacy, RouteOutcome: aicapability.RouteOutcomeLegacy},
		decision: aicapability.RouteDecision{
			Capability: aicapability.CapabilityProductEnrichText, Operation: aicapability.OperationProductEnrichTextExtract,
			ProviderID: "openai", ModelID: "legacy-model", RoutingKey: "default", CredentialReference: "default",
		},
		prompt: "prompt", input: "prompt",
		call:     func(context.Context) (string, error) { return "legacy response", nil },
		recorder: recorder,
		onRecordError: func(_ aicapability.InvocationRecord, err error) {
			if !errors.Is(err, recordErr) {
				t.Fatalf("callback error = %v", err)
			}
			callbackCalls++
		},
	}

	got, err := execution.invoke(context.Background(), aicapability.CacheStatusNotApplicable)
	if err != nil || got != "legacy response" {
		t.Fatalf("invoke = %q, %v", got, err)
	}
	if callbackCalls != 1 {
		t.Fatalf("callback calls = %d, want 1", callbackCalls)
	}
}

func TestPreparedExecutionUsesDynamicScoringPromptIdentityForCacheAndAudit(t *testing.T) {
	const renderedPrompt = "rendered prompt with raw marker API_KEY=sk-test-secret credential-material=secret-value"
	recorder := &preparedExecutionRecorder{}
	manager := &preparedScoringManager{}
	generator := &governedTextGenerator{
		manager: manager, planner: preparedScoringPlanner{}, recorder: recorder,
		capability: aicapability.CapabilityProductEnrichText, operation: aicapability.OperationProductEnrichTextQualityScore,
		requiredFeature: aicapability.FeatureTextGenerate,
		promptKey:       "stale-key", promptVersion: "v1", promptScope: "stale-scope",
	}
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
	execution, err := generator.PrepareText(ctx, renderedPrompt, productenrich.ScorePromptIdentity{
		PromptKey: "productenrich.llm_scorer.text_scoring", PromptVersion: "prompt-v17", PromptScope: "product_enrich",
	})
	if err != nil {
		t.Fatalf("PrepareText: %v", err)
	}

	identity := execution.ScoreCacheIdentity("80", "raw-input-hash")
	if identity.PromptKey != "productenrich.llm_scorer.text_scoring" || identity.PromptVersion != "prompt-v17" || identity.PromptScope != "product_enrich" {
		t.Fatalf("cache prompt identity = %+v", identity)
	}
	identityPromptHash := identity.PromptHash
	if identityPromptHash != hashText(renderedPrompt) {
		t.Fatalf("cache prompt hash = %q, want exact rendered prompt hash", identityPromptHash)
	}
	if !strings.HasPrefix(identity.Key(), "llm_score:governed:v2:") {
		t.Fatalf("cache key = %q, want v2 namespace", identity.Key())
	}
	for _, forbidden := range []string{renderedPrompt, "sk-test-secret", "secret-value"} {
		if strings.Contains(identity.Key(), forbidden) {
			t.Fatalf("cache key leaked forbidden content %q: %q", forbidden, identity.Key())
		}
	}
	if _, err := execution.Invoke(ctx, aicapability.CacheStatusMiss); err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if err := execution.RecordCacheHit(ctx, "91"); err != nil {
		t.Fatalf("RecordCacheHit: %v", err)
	}
	if len(recorder.records) != 2 {
		t.Fatalf("records = %d, want miss and hit", len(recorder.records))
	}
	miss, hit := recorder.records[0], recorder.records[1]
	if miss.CacheStatus != aicapability.CacheStatusMiss || hit.CacheStatus != aicapability.CacheStatusHit {
		t.Fatalf("cache statuses = %q/%q", miss.CacheStatus, hit.CacheStatus)
	}
	if miss.PromptVersion != "prompt-v17" || hit.PromptVersion != "prompt-v17" || hit.PromptKey != "productenrich.llm_scorer.text_scoring" || hit.PromptScope != "product_enrich" {
		t.Fatalf("miss/hit prompt metadata = %+v / %+v", miss, hit)
	}
	if miss.PromptHash != identityPromptHash || hit.PromptHash != identityPromptHash {
		t.Fatalf("cache identity / ledger prompt hashes diverged: identity=%q miss=%q hit=%q", identityPromptHash, miss.PromptHash, hit.PromptHash)
	}
	encodedRecords, err := json.Marshal(recorder.records)
	if err != nil {
		t.Fatalf("marshal invocation records: %v", err)
	}
	for _, forbidden := range []string{renderedPrompt, "sk-test-secret", "secret-value"} {
		if strings.Contains(string(encodedRecords), forbidden) {
			t.Fatalf("invocation record leaked forbidden content %q: %s", forbidden, encodedRecords)
		}
	}
	if hit.RouteMode != aicapability.RoutingModeActive || hit.RouteOutcome != aicapability.RouteOutcomeActive || hit.ProviderID != "openai" || hit.ModelID != "score-model" || hit.RoutingKey != "fast" {
		t.Fatalf("cache-hit route metadata = %+v", hit)
	}
	if hit.PromptTokens != 0 || hit.CompletionTokens != 0 || hit.TotalTokens != 0 || hit.EstimatedCostMicros != 0 {
		t.Fatalf("cache-hit usage must be zero: %+v", hit)
	}
	if manager.providerCalls != 1 {
		t.Fatalf("provider calls = %d, want only the miss invocation", manager.providerCalls)
	}
}

type preparedScoringPlanner struct{}

func (preparedScoringPlanner) Plan(context.Context, aicapability.RouteRequest) (aicapability.ExecutionPlan, error) {
	return aicapability.ExecutionPlan{
		Mode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive,
		Decision: aicapability.RouteDecision{
			Capability: aicapability.CapabilityProductEnrichText, Operation: aicapability.OperationProductEnrichTextQualityScore,
			ProviderID: "openai", ModelID: "score-model", RoutingKey: "fast", CredentialReference: "fast",
			PolicyVersion: "policy-v1", ConfigurationVersion: "config-v1",
		},
	}, nil
}

type preparedScoringManager struct {
	providerCalls int
}

func (*preparedScoringManager) GetClient(string) (productenrich.LLMClient, error) {
	return nil, errors.New("legacy lookup not expected")
}

func (*preparedScoringManager) GetDefaultClient() productenrich.LLMClient { return nil }

func (m *preparedScoringManager) GetClientWithRoute(context.Context, string, productenrich.LLMClientRoute) (productenrich.LLMClient, error) {
	return preparedScoringClient{manager: m}, nil
}

type preparedScoringClient struct {
	manager *preparedScoringManager
}

func (c preparedScoringClient) Generate(context.Context, string) (string, error) {
	c.manager.providerCalls++
	return `{"score":91}`, nil
}

func (preparedScoringClient) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", errors.New("image call not expected")
}

type preparedExecutionRecorder struct {
	records []aicapability.InvocationRecord
	err     error
}

func (r *preparedExecutionRecorder) RecordInvocation(_ context.Context, record aicapability.InvocationRecord) error {
	r.records = append(r.records, record)
	return r.err
}

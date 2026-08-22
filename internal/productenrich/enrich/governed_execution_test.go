package enrich

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/aicapability"
	"task-processor/internal/shared/aiidentity"
)

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

type preparedExecutionRecorder struct {
	records []aicapability.InvocationRecord
	err     error
}

func (r *preparedExecutionRecorder) RecordInvocation(_ context.Context, record aicapability.InvocationRecord) error {
	r.records = append(r.records, record)
	return r.err
}

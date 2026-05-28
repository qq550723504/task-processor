package listingkit

import (
	"testing"
	"time"
)

func TestEnsureTaskPodExecutionMarksRequiredSDSFailureAsBlocking(t *testing.T) {
	t.Parallel()

	task := &Task{
		Request: &GenerateRequest{
			Platforms: []string{"shein"},
			Options: &GenerateOptions{
				SDS: &SDSSyncOptions{
					VariantID: 901,
				},
			},
		},
		Result: &ListingKitResult{
			SDSDesignResult: &SDSSyncSummary{
				VariantID: 901,
				Status:    "failed",
				Error:     "design template unavailable",
			},
		},
	}

	if !ensureTaskPodExecution(task) {
		t.Fatal("ensureTaskPodExecution() = false, want updated pod execution")
	}
	if task.Result.PodExecution == nil {
		t.Fatal("pod execution = nil")
	}
	if task.Result.PodExecution.Provider != podProviderSDS {
		t.Fatalf("provider = %q, want %q", task.Result.PodExecution.Provider, podProviderSDS)
	}
	if task.Result.PodExecution.DependencyMode != podDependencyModeRequired {
		t.Fatalf("dependency mode = %q, want %q", task.Result.PodExecution.DependencyMode, podDependencyModeRequired)
	}
	if task.Result.PodExecution.Status != podStatusFailedBlocking {
		t.Fatalf("status = %q, want %q", task.Result.PodExecution.Status, podStatusFailedBlocking)
	}
	if len(task.Result.PodExecution.History) != 2 {
		t.Fatalf("history length = %d, want 2 audit events", len(task.Result.PodExecution.History))
	}
	if task.Result.PodExecution.History[0].Kind != podAuditKindPolicyDecision {
		t.Fatalf("first audit kind = %q, want %q", task.Result.PodExecution.History[0].Kind, podAuditKindPolicyDecision)
	}
	if task.Result.PodExecution.History[1].Kind != podAuditKindStatusChange {
		t.Fatalf("second audit kind = %q, want %q", task.Result.PodExecution.History[1].Kind, podAuditKindStatusChange)
	}
	if task.Result.PodExecution.History[1].ToStatus != podStatusFailedBlocking {
		t.Fatalf("status transition to_status = %q, want %q", task.Result.PodExecution.History[1].ToStatus, podStatusFailedBlocking)
	}
	readiness := buildSheinSubmitReadinessWithPodForAction(&SheinPackage{}, task.Result.PodExecution, "publish")
	if readiness == nil || readiness.Ready {
		t.Fatalf("readiness = %+v, want blocked readiness", readiness)
	}
	found := false
	for _, item := range readiness.BlockingItems {
		if item.Key == "pod_platform" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("blocking items = %+v, want pod_platform blocker", readiness.BlockingItems)
	}
}

func TestEnsureTaskPodExecutionDisablesPODWhenNoPlatformDependency(t *testing.T) {
	t.Parallel()

	task := &Task{
		Request: &GenerateRequest{
			Platforms: []string{"shein"},
			Options:   &GenerateOptions{},
		},
		Result: &ListingKitResult{},
	}

	ensureTaskPodExecution(task)
	if task.Result.PodExecution == nil {
		t.Fatal("pod execution = nil")
	}
	if task.Result.PodExecution.DependencyMode != podDependencyModeDisabled {
		t.Fatalf("dependency mode = %q, want %q", task.Result.PodExecution.DependencyMode, podDependencyModeDisabled)
	}
	if task.Result.PodExecution.Status != podStatusNotApplicable {
		t.Fatalf("status = %q, want %q", task.Result.PodExecution.Status, podStatusNotApplicable)
	}
	if len(task.Result.PodExecution.History) != 2 {
		t.Fatalf("history length = %d, want initial policy and status audit", len(task.Result.PodExecution.History))
	}
}

func TestEnsureTaskPodExecutionMarksOptionalAIPODFailureAsDegraded(t *testing.T) {
	t.Parallel()

	task := &Task{
		Request: &GenerateRequest{
			Platforms: []string{"shein"},
			ImageURLs: []string{"https://cdn.example.com/source.png"},
			Options: &GenerateOptions{
				ProcessImages: false,
				ImageStrategy: sheinImageStrategyAIGenerated,
				SheinStudio: &SheinStudioOptions{
					RenderSizeImagesWithSDS: true,
				},
				SDS: &SDSSyncOptions{
					VariantID: 901,
				},
			},
		},
		Result: &ListingKitResult{
			SDSDesignResult: &SDSSyncSummary{
				VariantID: 901,
				Status:    "failed",
				Error:     "size image render unavailable",
			},
		},
	}

	ensureTaskPodExecution(task)
	if task.Result.PodExecution == nil {
		t.Fatal("pod execution = nil")
	}
	if task.Result.PodExecution.DependencyMode != podDependencyModeOptional {
		t.Fatalf("dependency mode = %q, want %q", task.Result.PodExecution.DependencyMode, podDependencyModeOptional)
	}
	if task.Result.PodExecution.Status != podStatusFailedDegraded {
		t.Fatalf("status = %q, want %q", task.Result.PodExecution.Status, podStatusFailedDegraded)
	}
	if len(task.Result.PodExecution.History) != 2 {
		t.Fatalf("history length = %d, want 2 audit events", len(task.Result.PodExecution.History))
	}
	if task.Result.PodExecution.History[1].ToStatus != podStatusFailedDegraded {
		t.Fatalf("status transition to_status = %q, want %q", task.Result.PodExecution.History[1].ToStatus, podStatusFailedDegraded)
	}
	readiness := buildSheinSubmitReadinessWithPodForAction(&SheinPackage{}, task.Result.PodExecution, "publish")
	if readiness == nil {
		t.Fatal("readiness = nil")
	}
	for _, item := range readiness.BlockingItems {
		if item.Key == "pod_platform" {
			t.Fatalf("blocking items = %+v, want pod_platform not to block optional fallback", readiness.BlockingItems)
		}
	}
	found := false
	for _, item := range readiness.WarningItems {
		if item.Key == "pod_platform" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("warning items = %+v, want pod_platform warning", readiness.WarningItems)
	}
}

func TestEnsureTaskPodExecutionDoesNotDuplicateAuditHistoryWhenStateIsStable(t *testing.T) {
	t.Parallel()

	task := &Task{
		Request: &GenerateRequest{
			Platforms: []string{"shein"},
			Options: &GenerateOptions{
				SDS: &SDSSyncOptions{
					VariantID: 901,
				},
			},
		},
		Result: &ListingKitResult{
			SDSDesignResult: &SDSSyncSummary{
				VariantID: 901,
				Status:    "failed",
				Error:     "design template unavailable",
			},
		},
	}

	ensureTaskPodExecution(task)
	firstHistoryLen := len(task.Result.PodExecution.History)
	if firstHistoryLen == 0 {
		t.Fatal("history length = 0, want audit events after first derivation")
	}
	if ensureTaskPodExecution(task) {
		t.Fatal("ensureTaskPodExecution() = true, want no semantic change on stable state")
	}
	if len(task.Result.PodExecution.History) != firstHistoryLen {
		t.Fatalf("history length = %d, want unchanged %d", len(task.Result.PodExecution.History), firstHistoryLen)
	}
}

func TestMarkPodExecutionStatusRecordsProcessingTransition(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		PodExecution: &PodExecutionSummary{
			Provider:       podProviderSDS,
			DependencyMode: podDependencyModeRequired,
			Status:         podStatusPending,
			History: []PodExecutionAuditEvent{{
				Kind:           podAuditKindPolicyDecision,
				Code:           podAuditCodePolicyApplied,
				Provider:       podProviderSDS,
				DependencyMode: podDependencyModeRequired,
				DecisionSource: "system_rule",
			}},
		},
	}

	markPodExecutionStatus(result, podStatusProcessing, time.Time{})

	if result.PodExecution == nil {
		t.Fatal("pod execution = nil")
	}
	if result.PodExecution.Status != podStatusProcessing {
		t.Fatalf("status = %q, want %q", result.PodExecution.Status, podStatusProcessing)
	}
	if len(result.PodExecution.History) != 2 {
		t.Fatalf("history length = %d, want appended transition event", len(result.PodExecution.History))
	}
	last := result.PodExecution.History[len(result.PodExecution.History)-1]
	if last.Kind != podAuditKindStatusChange || last.ToStatus != podStatusProcessing {
		t.Fatalf("last audit event = %+v, want processing transition", last)
	}
}

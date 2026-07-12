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
			ImageURLs: []string{"https://cdn.example.com/source.png"},
			Options: &GenerateOptions{
				ProcessImages: false,
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
			ImageURLs: []string{"https://cdn.example.com/source.png"},
			Options: &GenerateOptions{
				ProcessImages: false,
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

func TestDerivePodExecutionSummaryUsesProcessingChildTaskOverStaleSDSFailure(t *testing.T) {
	t.Parallel()

	result := derivePodExecutionSummary(
		&PodExecutionSummary{
			Provider:       podProviderSDS,
			DependencyMode: podDependencyModeRequired,
			Status:         podStatusFailedBlocking,
			FailureReason:  "previous render timeout",
			FallbackType:   podFallbackLocalMockup,
		},
		&SDSSyncSummary{
			VariantID: 901,
			Status:    "failed",
			Error:     "previous render timeout",
		},
		[]ChildTaskState{{
			Kind:   "sds_design_sync",
			Status: string(TaskStatusProcessing),
		}},
		&GenerateRequest{
			Platforms: []string{"shein"},
			ImageURLs: []string{"https://cdn.example.com/source.png"},
			Options: &GenerateOptions{
				ProcessImages: false,
				SDS: &SDSSyncOptions{
					VariantID: 901,
				},
			},
		},
		time.Time{},
	)

	if result.Status != podStatusProcessing {
		t.Fatalf("status = %q, want %q", result.Status, podStatusProcessing)
	}
	if result.FailureReason != "" {
		t.Fatalf("failure reason = %q, want empty after retry starts", result.FailureReason)
	}
	if result.FallbackType != "" {
		t.Fatalf("fallback type = %q, want empty after retry starts", result.FallbackType)
	}
	if result.CompletedAt != nil {
		t.Fatalf("completed at = %v, want nil while processing", result.CompletedAt)
	}
}

func TestDerivePodExecutionSummaryClearsFailureDetailsAfterSuccess(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	result := derivePodExecutionSummary(
		&PodExecutionSummary{
			Provider:       podProviderSDS,
			DependencyMode: podDependencyModeRequired,
			Status:         podStatusFailedBlocking,
			FailureReason:  "previous render timeout",
			FallbackType:   podFallbackLocalMockup,
		},
		&SDSSyncSummary{
			VariantID: 901,
			Status:    "completed",
		},
		nil,
		&GenerateRequest{
			Platforms: []string{"shein"},
			ImageURLs: []string{"https://cdn.example.com/source.png"},
			Options: &GenerateOptions{
				ProcessImages: false,
				SDS: &SDSSyncOptions{
					VariantID: 901,
				},
			},
		},
		updatedAt,
	)

	if result.Status != podStatusSucceeded {
		t.Fatalf("status = %q, want %q", result.Status, podStatusSucceeded)
	}
	if result.FailureReason != "" {
		t.Fatalf("failure reason = %q, want empty after success", result.FailureReason)
	}
	if result.FallbackType != "" {
		t.Fatalf("fallback type = %q, want empty after success", result.FallbackType)
	}
	if result.CompletedAt == nil || !result.CompletedAt.Equal(updatedAt) {
		t.Fatalf("completed at = %v, want %v", result.CompletedAt, updatedAt)
	}
}

func TestDerivePodExecutionSummaryClearsSDSFailureDetailsAfterSuccess(t *testing.T) {
	updatedAt := time.Date(2026, 7, 12, 9, 0, 0, 0, time.UTC)
	result := derivePodExecutionSummary(
		&PodExecutionSummary{
			Provider:       podProviderSDS,
			DependencyMode: podDependencyModeRequired,
			Status:         podStatusFailedBlocking,
			FailureReason:  "old timeout",
			FallbackType:   podFallbackLocalMockup,
		},
		&SDSSyncSummary{Status: "completed", Error: "old timeout"},
		nil,
		&GenerateRequest{
			Platforms: []string{"shein"},
			ImageURLs: []string{"https://cdn.example.com/source.png"},
			Options:   &GenerateOptions{SDS: &SDSSyncOptions{VariantID: 901}},
		},
		updatedAt,
	)
	if result.Status != podStatusSucceeded || result.FailureReason != "" || result.FallbackType != "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestMarkPodExecutionStatusRecordsProcessingTransition(t *testing.T) {
	t.Parallel()

	result := &ListingKitResult{
		PodExecution: &PodExecutionSummary{
			Provider:       podProviderSDS,
			DependencyMode: podDependencyModeRequired,
			Status:         podStatusPending,
			FailureReason:  "previous render timeout",
			FallbackType:   podFallbackLocalMockup,
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
	if result.PodExecution.FailureReason != "" {
		t.Fatalf("failure reason = %q, want empty after processing transition", result.PodExecution.FailureReason)
	}
	if result.PodExecution.FallbackType != "" {
		t.Fatalf("fallback type = %q, want empty after processing transition", result.PodExecution.FallbackType)
	}
	if len(result.PodExecution.History) != 2 {
		t.Fatalf("history length = %d, want appended transition event", len(result.PodExecution.History))
	}
	last := result.PodExecution.History[len(result.PodExecution.History)-1]
	if last.Kind != podAuditKindStatusChange || last.ToStatus != podStatusProcessing {
		t.Fatalf("last audit event = %+v, want processing transition", last)
	}
}

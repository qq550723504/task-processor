package sdspod

import "testing"

func TestDeriveExecutionClearsHistoricalFailureDetailsAfterSuccess(t *testing.T) {
	result := DeriveExecution(DeriveInput{
		Current: Execution{
			Provider:       ProviderSDS,
			DependencyMode: DependencyRequired,
			Status:         StatusFailedBlocking,
			FailureReason:  "old timeout",
			FallbackType:   FallbackLocalMockup,
		},
		SDS: &SDSResult{Status: "completed", Error: "old timeout"},
	})
	if result.Status != StatusSucceeded || result.FailureReason != "" || result.FallbackType != "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestDeriveExecutionPrioritizesActiveChildOverStaleSDSFailure(t *testing.T) {
	result := DeriveExecution(DeriveInput{
		Current:  Execution{Provider: ProviderSDS, DependencyMode: DependencyRequired},
		SDS:      &SDSResult{Status: "failed", Error: "old failure"},
		Children: []ChildTask{{Kind: SDSDesignSyncKind, Status: "processing"}},
	})
	if result.Status != StatusProcessing || result.FailureReason != "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestSubmissionBlockedAndReadinessMessageRespectDependencyMode(t *testing.T) {
	cases := []struct {
		name      string
		execution Execution
		blocked   bool
		message   string
	}{
		{
			name:      "required failure",
			execution: Execution{Provider: ProviderSDS, DependencyMode: DependencyRequired, Status: StatusFailedBlocking, FailureReason: "timeout"},
			blocked:   true,
			message:   "SDS 平台处理为发布前置，当前不可提交：timeout",
		},
		{
			name:      "optional fallback",
			execution: Execution{Provider: ProviderSDS, DependencyMode: DependencyOptional, Status: StatusFailedDegraded, FailureReason: "timeout"},
			blocked:   false,
			message:   "SDS 平台处理失败，当前将按降级素材继续发布：timeout",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := SubmissionBlocked(tt.execution); got != tt.blocked {
				t.Fatalf("blocked = %t, want %t", got, tt.blocked)
			}
			if got := ReadinessMessage(tt.execution); got != tt.message {
				t.Fatalf("message = %q, want %q", got, tt.message)
			}
		})
	}
}

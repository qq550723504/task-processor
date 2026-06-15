package submission

import (
	"testing"
	"time"
)

func TestBuildSubmitAttemptPlanDerivesWorkflowRequestID(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 6, 9, 14, 0, 0, 0, time.FixedZone("CST", 8*3600))
	plan := BuildSubmitAttemptPlan("task-123", "shein", "publish", "", "", startedAt, func(platform, action string) bool {
		return platform == "shein" && action == "publish"
	})

	if !plan.UseWorkflow {
		t.Fatal("UseWorkflow = false, want true")
	}
	if plan.RequestID == "" {
		t.Fatal("RequestID = empty, want derived workflow request id")
	}
	if plan.RequestID != DeriveWorkflowRequestID("task-123", "publish", startedAt) {
		t.Fatalf("RequestID = %q, want derived workflow request id", plan.RequestID)
	}
	if !plan.StartedAt.Equal(startedAt) {
		t.Fatalf("StartedAt = %v, want %v", plan.StartedAt, startedAt)
	}
}

func TestBuildSubmitAttemptPlanKeepsExplicitRequestID(t *testing.T) {
	t.Parallel()

	plan := BuildSubmitAttemptPlan("task-123", "shein", "publish", "explicit-req-1", "", time.Now(), func(string, string) bool {
		return true
	})
	if plan.RequestID != "explicit-req-1" {
		t.Fatalf("RequestID = %q, want explicit-req-1", plan.RequestID)
	}
}

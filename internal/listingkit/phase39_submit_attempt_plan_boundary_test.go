package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSubmitAttemptPlanBoundary(t *testing.T) {
	t.Parallel()

	submissionSrc, err := os.ReadFile("../listing/submission/submit_attempt_plan.go")
	if err != nil {
		t.Fatalf("ReadFile(../listing/submission/submit_attempt_plan.go) error = %v", err)
	}
	submissionContent := string(submissionSrc)

	for _, needle := range []string{
		"type SubmitAttemptPlan struct {",
		"func BuildSubmitAttemptPlan(",
		"resolvedRequestID := ResolveSubmitRequestID(idempotencyKey, requestID)",
		"resolvedRequestID = DeriveWorkflowRequestID(taskID, action, startedAt)",
	} {
		if !strings.Contains(submissionContent, needle) {
			t.Fatalf("submit_attempt_plan.go should contain %q", needle)
		}
	}

	serviceSrc, err := os.ReadFile("task_submission_service.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_service.go) error = %v", err)
	}
	serviceContent := string(serviceSrc)

	for _, needle := range []string{
		"plan := listingsubmission.BuildSubmitAttemptPlan(taskID, platform, action, idempotencyKey, explicitRequestID, startedAt, shouldStartWorkflow)",
		"platform:    plan.Platform,",
		"requestID:   plan.RequestID,",
		"useWorkflow: plan.UseWorkflow,",
	} {
		if !strings.Contains(serviceContent, needle) {
			t.Fatalf("task_submission_service.go should delegate attempt plan seam via %q", needle)
		}
	}
}

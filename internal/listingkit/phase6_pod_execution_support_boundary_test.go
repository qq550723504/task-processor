package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestPodExecutionSupportBoundary(t *testing.T) {
	t.Parallel()

	rootPath := "pod_execution.go"
	policyPath := "pod_execution_policy_support.go"
	auditPath := "pod_execution_audit_support.go"

	rootContent, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatalf("read root file: %v", err)
	}
	policyContent, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("read policy support file: %v", err)
	}
	auditContent, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit support file: %v", err)
	}

	rootText := string(rootContent)
	policyText := string(policyContent)
	auditText := string(auditContent)

	for _, snippet := range []string{
		"func normalizePodExecutionSummary(pod *PodExecutionSummary) *PodExecutionSummary {",
		"func ensureResultPodExecution(result *ListingKitResult, req *GenerateRequest) bool {",
		"func derivePodExecutionSummary(current *PodExecutionSummary, sds *SDSSyncSummary, childTasks []ChildTaskState, req *GenerateRequest, updatedAt time.Time) *PodExecutionSummary {",
		"func markPodExecutionStatus(result *ListingKitResult, status string, at time.Time) {",
	} {
		if !strings.Contains(rootText, snippet) {
			t.Fatalf("root file missing seam %q", snippet)
		}
	}

	for _, snippet := range []string{
		"func inferPodStatusFromSDS(sds *SDSSyncSummary, childTasks []ChildTaskState, dependencyMode string) (status string, reason string, fallback string, found bool) {",
		"func podSubmissionBlocked(pod *PodExecutionSummary) bool {",
		"func podReadinessMessage(pod *PodExecutionSummary) string {",
	} {
		if strings.Contains(rootText, snippet) {
			t.Fatalf("root file still contains policy helper %q", snippet)
		}
		if !strings.Contains(policyText, snippet) {
			t.Fatalf("policy support file missing helper %q", snippet)
		}
	}

	for _, snippet := range []string{
		"func podExecutionEqual(left *PodExecutionSummary, right *PodExecutionSummary) bool {",
		"func normalizePodExecutionAuditHistory(items []PodExecutionAuditEvent) []PodExecutionAuditEvent {",
		"func recordPodExecutionAudit(before *PodExecutionSummary, after *PodExecutionSummary, updatedAt time.Time) {",
	} {
		if strings.Contains(rootText, snippet) {
			t.Fatalf("root file still contains audit helper %q", snippet)
		}
		if !strings.Contains(auditText, snippet) {
			t.Fatalf("audit support file missing helper %q", snippet)
		}
	}
}

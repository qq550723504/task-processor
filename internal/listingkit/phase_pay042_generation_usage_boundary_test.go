package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestPay042GenerationUsageBoundary(t *testing.T) {
	t.Parallel()

	usageSource := readPay042Source(t, "usage_settlement.go")
	if strings.Contains(strings.ToLower(usageSource), "openmeter") || strings.Contains(strings.ToLower(usageSource), "payment") {
		t.Fatal("generation usage port must remain provider- and payment-independent")
	}
	for _, needle := range []string{
		"generationUsageMetric",
		"studio_design_jobs_succeeded",
		"generationUsageSourceType",
		"listingkit_generation",
		"type GenerationUsageSettlement interface",
	} {
		if !strings.Contains(usageSource, needle) {
			t.Fatalf("usage_settlement.go must contain %q", needle)
		}
	}

	runnerSource := readPay042Source(t, "service_process_runner_helper.go")
	reserveAt := strings.Index(runnerSource, "reserveGenerationUsage")
	workflowAt := strings.Index(runnerSource, "runWorkflow")
	if reserveAt == -1 || workflowAt == -1 || reserveAt > workflowAt {
		t.Fatal("generation reservation must be wired before workflow execution")
	}

	handlerSource, err := os.ReadFile("api/handler_tasks.go")
	if err != nil {
		t.Fatalf("read api/handler_tasks.go: %v", err)
	}
	for _, needle := range []string{"ReserveUsage(", "CommitUsage(", "RecordUsage("} {
		if strings.Contains(string(handlerSource), needle) {
			t.Fatalf("generation HTTP handler must not settle usage directly: found %q", needle)
		}
	}
}

func readPay042Source(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(content)
}

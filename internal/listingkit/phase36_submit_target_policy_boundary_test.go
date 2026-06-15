package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSubmitTargetPolicyBoundary(t *testing.T) {
	t.Parallel()

	submissionSrc, err := os.ReadFile("../listing/submission/submit_target.go")
	if err != nil {
		t.Fatalf("ReadFile(../listing/submission/submit_target.go) error = %v", err)
	}
	submissionContent := string(submissionSrc)

	for _, needle := range []string{
		"type SubmitTarget struct {",
		"func ResolveSubmitTarget(requestedPlatform, requestedAction, defaultPlatform, defaultAction string) SubmitTarget {",
		"func IsReplayOfStartedSubmit(err error, requestID string) bool {",
	} {
		if !strings.Contains(submissionContent, needle) {
			t.Fatalf("submit_target.go should contain %q", needle)
		}
	}

	contractsSrc, err := os.ReadFile("service_submit_contracts.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_contracts.go) error = %v", err)
	}
	contractsContent := string(contractsSrc)

	for _, needle := range []string{
		`target := listingsubmission.ResolveSubmitTarget(requestedPlatform, requestedAction, "shein", defaultAction)`,
		"return listingsubmission.IsReplayOfStartedSubmit(err, requestID)",
	} {
		if !strings.Contains(contractsContent, needle) {
			t.Fatalf("service_submit_contracts.go should contain %q", needle)
		}
	}
}

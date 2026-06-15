package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSubmitActionPolicyBoundary(t *testing.T) {
	t.Parallel()

	submissionSrc, err := os.ReadFile("../listing/submission/submit_action.go")
	if err != nil {
		t.Fatalf("ReadFile(../listing/submission/submit_action.go) error = %v", err)
	}
	submissionContent := string(submissionSrc)

	for _, needle := range []string{
		"func NormalizeSubmitAction(action, fallback string) string {",
		"func IsSupportedSubmitAction(action string) bool {",
	} {
		if !strings.Contains(submissionContent, needle) {
			t.Fatalf("submit_action.go should contain %q", needle)
		}
	}

	sharedSrc, err := os.ReadFile("service_submit_shared.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_shared.go) error = %v", err)
	}
	sharedContent := string(sharedSrc)

	if !strings.Contains(sharedContent, "return listingsubmission.IsSupportedSubmitAction(action)") {
		t.Fatalf("service_submit_shared.go should delegate submit action support policy to internal/listing/submission")
	}
}

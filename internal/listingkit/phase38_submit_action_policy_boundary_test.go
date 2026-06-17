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
		"func PreferredSubmitAction(candidates ...string) string {",
		"func UnsupportedSubmitActionError(action string) error {",
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

	for _, forbidden := range []string{
		"func derivedSheinSubmitRequestID(",
		"func isSupportedSubmitAction(",
		"func unsupportedSubmitActionError(",
	} {
		if strings.Contains(sharedContent, forbidden) {
			t.Fatalf("service_submit_shared.go should not keep root submission policy wrapper %q; call internal/listing/submission directly", forbidden)
		}
	}

	helperSrc, err := os.ReadFile("service_submit_action_normalization_helper.go")
	if err != nil {
		t.Fatalf("ReadFile(service_submit_action_normalization_helper.go) error = %v", err)
	}
	helperContent := string(helperSrc)
	if !strings.Contains(helperContent, "listingsubmission.PreferredSubmitAction(") {
		t.Fatalf("service_submit_action_normalization_helper.go should delegate preferred action selection to internal/listing/submission")
	}
	if strings.Contains(helperContent, "func normalizePreferredSheinSubmitAction(") {
		t.Fatalf("service_submit_action_normalization_helper.go should not own preferred submit action normalization")
	}
}

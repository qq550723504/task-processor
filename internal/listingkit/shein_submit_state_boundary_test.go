package listingkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSheinSubmitStateKeepsTransitionSequencingBoundary(t *testing.T) {
	t.Parallel()

	path := filepath.Join("shein_submit_state.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	content := string(data)

	forbidden := []string{
		"&sheinpub.SubmissionRecord{",
		"ResponseOutcome{",
		"func responseOutcome(",
		"func resolveSheinSubmitFailureState(",
		"func ensureSheinSubmissionReport(",
		"func setSheinSubmitRemoteRecord(",
		"func findSheinSubmissionRecordByRequestID(",
		"func findActiveSheinSubmitAttempt(",
		"func sheinSubmitAttemptNeedsRemoteRecovery(",
		"func sheinSubmissionRecordForAction(",
		"func clearSheinSubmitInFlight(",
		"func completeSheinSubmitAttemptAndBuildEvent(",
		"func failSheinSubmitAttemptAndBuildEvent(",
		"func failSheinSubmitAttemptWithResponseAndBuildEvent(",
	}
	for _, pattern := range forbidden {
		if strings.Contains(content, pattern) {
			t.Fatalf("%s should not contain %q; pure SHEIN record/outcome shaping belongs in internal/publishing/shein", path, pattern)
		}
	}
}

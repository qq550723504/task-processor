package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinStoreResolutionBoundary(t *testing.T) {
	t.Parallel()

	fileSrc, err := os.ReadFile("shein_store_resolution_presentation.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_store_resolution_presentation.go) error = %v", err)
	}
	if !strings.Contains(string(fileSrc), `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
		t.Fatal("shein_store_resolution_presentation.go should call marketplace SHEIN workspace directly")
	}

	source := readNamedFunctionSource(t, "shein_store_resolution_presentation.go", "buildSheinStoreResolutionSummaryValue")
	callNames := readNamedFunctionCallNames(t, "shein_store_resolution_presentation.go", "buildSheinStoreResolutionSummaryValue")

	assertSourceContainsAll(t, source, []string{
		"return sheinworkspace.BuildStoreResolutionSummary(",
	})
	assertSourceExcludesAll(t, source, []string{
		"return &SheinStoreResolutionSummary{",
		"MatchedRuleKinds: append([]string(nil), matchedRuleKinds...)",
	})
	assertFunctionCallsContainAll(t, callNames, []string{
		"BuildStoreResolutionSummary",
	})
}

package listingkit

import "testing"

func TestSheinStoreResolutionBoundary(t *testing.T) {
	t.Parallel()

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

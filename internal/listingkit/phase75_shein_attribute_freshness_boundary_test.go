package listingkit

import "testing"

func TestSheinAttributeFreshnessBoundary(t *testing.T) {
	t.Parallel()

	t.Run("attribute_freshness_home_delegates_to_workspace_bridge", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_freshness_shein.go", "evaluateSheinAttributeFreshness")
		callNames := readNamedFunctionCallNames(t, "submit_freshness_shein.go", "evaluateSheinAttributeFreshness")

		assertSourceContainsAll(t, source, []string{
			"return sheinworkspace.EvaluateAttributeFreshness(current, templates)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"EvaluateAttributeFreshness",
		})
	})
}

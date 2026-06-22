package listingkit

import "testing"

func TestSheinAttributeFreshnessBoundary(t *testing.T) {
	t.Parallel()

	t.Run("attribute_freshness_flow_calls_workspace_bridge", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_freshness_shein_flow_support.go", "validateSheinFreshnessAttributes")
		callNames := readNamedFunctionCallNames(t, "submit_freshness_shein_flow_support.go", "validateSheinFreshnessAttributes")

		assertSourceContainsAll(t, source, []string{
			"attributeReady, attributeMessage := sheinworkspace.EvaluateAttributeFreshness(pkg, attributeTemplates)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"EvaluateAttributeFreshness",
		})

		homeSource := readTaskGenerationSourceFile(t, "submit_freshness_shein.go")
		assertSourceExcludesAll(t, homeSource, []string{
			"func evaluateSheinCategoryFreshness(",
			"func evaluateSheinAttributeFreshness(",
		})
	})
}

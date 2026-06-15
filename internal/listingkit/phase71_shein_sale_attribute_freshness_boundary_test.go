package listingkit

import "testing"

func TestSheinSaleAttributeFreshnessBoundary(t *testing.T) {
	t.Parallel()

	t.Run("sale_attribute_freshness_home_delegates_to_workspace_bridge", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_freshness_shein.go", "evaluateSheinSaleAttributeFreshnessWithCustomValidation")
		callNames := readNamedFunctionCallNames(t, "submit_freshness_shein.go", "evaluateSheinSaleAttributeFreshnessWithCustomValidation")

		assertSourceContainsAll(t, source, []string{
			"return sheinworkspace.EvaluateSaleAttributeFreshnessWithCustomValidation(current, templates, api)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"EvaluateSaleAttributeFreshnessWithCustomValidation",
		})
	})
}

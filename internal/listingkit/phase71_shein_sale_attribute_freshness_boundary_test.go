package listingkit

import "testing"

func TestSheinSaleAttributeFreshnessBoundary(t *testing.T) {
	t.Parallel()

	t.Run("sale_attribute_freshness_flow_calls_workspace_bridge", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_freshness_shein_flow_support.go", "validateSheinFreshnessSaleAttributes")
		callNames := readNamedFunctionCallNames(t, "submit_freshness_shein_flow_support.go", "validateSheinFreshnessSaleAttributes")

		assertSourceContainsAll(t, source, []string{
			"saleReady, saleMessage, saleChanged := sheinworkspace.EvaluateSaleAttributeFreshnessWithCustomValidation(pkg, state.attributeTemplates, state.saleAPI)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"EvaluateSaleAttributeFreshnessWithCustomValidation",
		})

		homeSource := readTaskGenerationSourceFile(t, "submit_freshness_shein.go")
		assertSourceExcludesAll(t, homeSource, []string{
			"func evaluateSheinSaleAttributeFreshness(",
			"func evaluateSheinSaleAttributeFreshnessWithCustomValidation(",
		})
	})
}

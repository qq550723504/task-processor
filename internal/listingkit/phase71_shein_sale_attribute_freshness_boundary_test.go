package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinSaleAttributeFreshnessBoundary(t *testing.T) {
	t.Parallel()

	t.Run("sale_attribute_freshness_flow_calls_marketplace_workspace", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_freshness_shein_flow_support.go", "validateSheinFreshnessSaleAttributes")
		callNames := readNamedFunctionCallNames(t, "submit_freshness_shein_flow_support.go", "validateSheinFreshnessSaleAttributes")
		fileSource, err := os.ReadFile("submit_freshness_shein_flow_support.go")
		if err != nil {
			t.Fatalf("ReadFile(submit_freshness_shein_flow_support.go) error = %v", err)
		}
		if !strings.Contains(string(fileSource), `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
			t.Fatal("submit_freshness_shein_flow_support.go should call marketplace SHEIN workspace directly")
		}
		assertFileAbsent(t, "workspace/shein/freshness_bridge.go")

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

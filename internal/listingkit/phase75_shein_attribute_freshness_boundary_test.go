package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinAttributeFreshnessBoundary(t *testing.T) {
	t.Parallel()

	t.Run("attribute_freshness_flow_calls_marketplace_workspace", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_freshness_shein_flow_support.go", "validateSheinFreshnessAttributes")
		callNames := readNamedFunctionCallNames(t, "submit_freshness_shein_flow_support.go", "validateSheinFreshnessAttributes")
		fileSource, err := os.ReadFile("submit_freshness_shein_flow_support.go")
		if err != nil {
			t.Fatalf("ReadFile(submit_freshness_shein_flow_support.go) error = %v", err)
		}
		if !strings.Contains(string(fileSource), `sheinworkspace "task-processor/internal/marketplace/shein/workspace"`) {
			t.Fatal("submit_freshness_shein_flow_support.go should call marketplace SHEIN workspace directly")
		}
		assertFileAbsent(t, "workspace/shein/freshness_bridge.go")

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

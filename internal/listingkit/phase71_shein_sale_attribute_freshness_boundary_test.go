package listingkit

import "testing"

func TestSheinSaleAttributeFreshnessBoundary(t *testing.T) {
	t.Parallel()

	t.Run("sale_attribute_freshness_home_routes_context_build_and_resolution_home_only", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_freshness_shein.go", "evaluateSheinSaleAttributeFreshnessWithCustomValidation")
		callNames := readNamedFunctionCallNames(t, "submit_freshness_shein.go", "evaluateSheinSaleAttributeFreshnessWithCustomValidation")

		assertSourceContainsAll(t, source, []string{
			"templateContext, ok := buildSheinSaleAttributeFreshnessTemplateContext(templates)",
			"return evaluateSheinSaleAttributeFreshnessResolution(current, currentResolution, templateContext, api)",
		})
		assertSourceExcludesAll(t, source, []string{
			"collectInvalidSaleAttributes(",
			"repairSheinFreshnessSaleAttributes(",
			"sort.Strings(invalidSKC)",
			"sort.Strings(invalidSKU)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildSheinSaleAttributeFreshnessTemplateContext",
			"evaluateSheinSaleAttributeFreshnessResolution",
		})
	})

	t.Run("resolution_home_routes_invalid_state_and_message_shape_homes_only", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_sale_attribute_freshness_evaluation_shein.go", "evaluateSheinSaleAttributeFreshnessResolution")
		callNames := readNamedFunctionCallNames(t, "submit_sale_attribute_freshness_evaluation_shein.go", "evaluateSheinSaleAttributeFreshnessResolution")

		assertSourceContainsAll(t, source, []string{
			"invalidState := evaluateSheinSaleAttributeFreshnessInvalidState(current, currentResolution, templateContext, api)",
			"return buildSheinSaleAttributeFreshnessResolutionOutcome(baseIssues, invalidState)",
		})
		assertSourceExcludesAll(t, source, []string{
			"collectInvalidSaleAttributes(",
			"repairSheinFreshnessSaleAttributes(",
			"sort.Strings(invalidState.invalidSKC)",
			"sort.Strings(invalidState.invalidSKU)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"evaluateSheinSaleAttributeFreshnessInvalidState",
			"buildSheinSaleAttributeFreshnessResolutionOutcome",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"buildSheinPublishRequestForTask",
			"loadSheinAttributeTemplatesForFreshness",
			"loadSheinCategoryInfoForFreshness",
		})
	})

	t.Run("invalid_state_home_owns_invalid_collection_and_repair_routing", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_sale_attribute_freshness_resolution_repair_shein.go", "evaluateSheinSaleAttributeFreshnessInvalidState")
		callNames := readNamedFunctionCallNames(t, "submit_sale_attribute_freshness_resolution_repair_shein.go", "evaluateSheinSaleAttributeFreshnessInvalidState")

		assertSourceContainsAll(t, source, []string{
			"collectInvalidSaleAttributes(currentResolution.SKCAttributes, templateContext.byID, customRelationIDs)",
			"collectInvalidSaleAttributes(currentResolution.SKUAttributes, templateContext.byID, customRelationIDs)",
			"repaired := repairSheinFreshnessSaleAttributes(current, templateContext.attrByID, api)",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"buildSheinPublishRequestForTask",
			"loadSheinAttributeTemplatesForFreshness",
			"loadSheinCategoryInfoForFreshness",
		})
	})

	t.Run("message_shape_home_owns_issue_aggregation_and_outward_messages", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_sale_attribute_freshness_message_shape_shein.go", "buildSheinSaleAttributeFreshnessResolutionOutcome")
		callNames := readNamedFunctionCallNames(t, "submit_sale_attribute_freshness_message_shape_shein.go", "buildSheinSaleAttributeFreshnessResolutionOutcome")

		assertSourceContainsAll(t, source, []string{
			"sort.Strings(invalidState.invalidSKC)",
			"sort.Strings(invalidState.invalidSKU)",
			"issues = append(issues, \"当前模板已失效的 SKC 销售属性值: \"+strings.Join(invalidState.invalidSKC, \"; \"))",
			"issues = append(issues, \"当前模板已失效的 SKU 销售属性值: \"+strings.Join(invalidState.invalidSKU, \"; \"))",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"collectInvalidSaleAttributes",
			"repairSheinFreshnessSaleAttributes",
			"buildSheinPublishRequestForTask",
			"loadSheinAttributeTemplatesForFreshness",
			"loadSheinCategoryInfoForFreshness",
		})
	})
}

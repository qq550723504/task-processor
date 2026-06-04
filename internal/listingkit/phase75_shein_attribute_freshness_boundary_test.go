package listingkit

import "testing"

func TestSheinAttributeFreshnessBoundary(t *testing.T) {
	t.Parallel()

	t.Run("attribute_freshness_home_routes_template_context_and_resolved_attribute_home_only", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_freshness_shein.go", "evaluateSheinAttributeFreshness")
		callNames := readNamedFunctionCallNames(t, "submit_freshness_shein.go", "evaluateSheinAttributeFreshness")

		assertSourceContainsAll(t, source, []string{
			"templateContext, ok := buildSheinAttributeFreshnessTemplateContext(current, templates)",
			"return evaluateSheinResolvedAttributeFreshness(current, templateContext)",
		})
		assertSourceExcludesAll(t, source, []string{
			"filterSheinFreshnessDisplayAttributes(",
			"buildResolvedAttributeTemplateDriftDetails(",
			"sort.Strings(invalid)",
			"sort.Strings(missingRequired)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildSheinAttributeFreshnessTemplateContext",
			"evaluateSheinResolvedAttributeFreshness",
		})
	})

	t.Run("resolved_attribute_home_routes_issue_state_and_message_shape_homes_only", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_attribute_freshness_evaluation_shein.go", "evaluateSheinResolvedAttributeFreshness")
		callNames := readNamedFunctionCallNames(t, "submit_attribute_freshness_evaluation_shein.go", "evaluateSheinResolvedAttributeFreshness")

		assertSourceContainsAll(t, source, []string{
			"issueState := evaluateSheinAttributeFreshnessIssueState(current, templateContext)",
			"return buildSheinAttributeFreshnessOutcome(issueState, templateContext)",
		})
		assertSourceExcludesAll(t, source, []string{
			"if drift := buildResolvedAttributeTemplateDriftDetails(issueState.invalidItems, templateContext.attributeIndex); drift != \"\" {",
			"sort.Strings(issueState.invalid)",
			"sort.Strings(issueState.missing)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"evaluateSheinAttributeFreshnessIssueState",
			"buildSheinAttributeFreshnessOutcome",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"buildSheinPublishRequestForTask",
			"loadSheinAttributeTemplatesForFreshness",
			"loadSheinCategoryInfoForFreshness",
			"evaluateSheinSaleAttributeFreshnessWithCustomValidation",
		})
	})

	t.Run("issue_state_home_owns_invalid_and_missing_required_collection", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_attribute_freshness_issue_state_shein.go", "evaluateSheinAttributeFreshnessIssueState")
		callNames := readNamedFunctionCallNames(t, "submit_attribute_freshness_issue_state_shein.go", "evaluateSheinAttributeFreshnessIssueState")

		assertSourceContainsAll(t, source, []string{
			"if sheinResolvedAttributeStillLegal(item, templateContext.attributeIndex) {",
			"if !sheinpubDependencyIsActive(attr, templateContext.resolvedByID) {",
			"missingRequired = append(missingRequired, formatSheinFreshnessAttributeName(attr))",
			"invalid = append(invalid, formatResolvedAttributeDiffItem(item))",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"buildSheinPublishRequestForTask",
			"loadSheinAttributeTemplatesForFreshness",
			"loadSheinCategoryInfoForFreshness",
			"evaluateSheinSaleAttributeFreshnessWithCustomValidation",
		})
	})

	t.Run("message_shape_home_owns_drift_detail_and_outward_message_assembly", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_attribute_freshness_message_shape_shein.go", "buildSheinAttributeFreshnessOutcome")
		callNames := readNamedFunctionCallNames(t, "submit_attribute_freshness_message_shape_shein.go", "buildSheinAttributeFreshnessOutcome")

		assertSourceContainsAll(t, source, []string{
			"sort.Strings(issueState.invalid)",
			"sort.Strings(issueState.missing)",
			"if drift := buildResolvedAttributeTemplateDriftDetails(issueState.invalidItems, templateContext.attributeIndex); drift != \"\" {",
			"parts = append(parts, \"当前模板新增或恢复生效的必填属性: \"+strings.Join(issueState.missing, \"; \"))",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"buildSheinPublishRequestForTask",
			"loadSheinAttributeTemplatesForFreshness",
			"loadSheinCategoryInfoForFreshness",
			"evaluateSheinSaleAttributeFreshnessWithCustomValidation",
		})
	})
}

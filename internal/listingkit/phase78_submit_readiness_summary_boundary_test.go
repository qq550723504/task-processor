package listingkit

import "testing"

func TestSheinSubmitReadinessSummaryBoundary(t *testing.T) {
	t.Parallel()

	t.Run("submit_readiness_home_delegates_summary_post_shape_to_shared_seam", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "shein_submit_readiness.go", "buildSheinSubmitReadinessWithPodForAction")
		callNames := readNamedFunctionCallNames(t, "shein_submit_readiness.go", "buildSheinSubmitReadinessWithPodForAction")

		assertSourceContainsAll(t, source, []string{
			"return shapeSheinSubmitReadinessSummary(readiness, sheinSubmitReadinessSummaryShape{",
			"blockingLabel: \"待补关键项：\"",
			"warningLabel:  \"待确认项：\"",
		})
		assertSourceExcludesAll(t, source, []string{
			"readiness.Summary = append(readiness.Summary, \"待补关键项：\"+",
			"readiness.Summary = append(readiness.Summary, \"待确认项：\"+",
			"readiness.Summary = uniqueStrings(readiness.Summary)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"shapeSheinSubmitReadinessSummary",
		})
	})

	t.Run("freshness_readiness_home_delegates_summary_post_shape_to_shared_seam", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "shein_submit_freshness.go", "buildSheinSubmitFreshnessReadiness")
		callNames := readNamedFunctionCallNames(t, "shein_submit_freshness.go", "buildSheinSubmitFreshnessReadiness")

		assertSourceContainsAll(t, source, []string{
			"return shapeSheinSubmitReadinessSummary(readiness, sheinSubmitReadinessSummaryShape{",
			"blockingLabel:       \"在线阻断项：\"",
			"prependFirstBlocker: true",
		})
		assertSourceExcludesAll(t, source, []string{
			"readiness.Summary = append([]string{message}, readiness.Summary...)",
			"readiness.Summary = append(readiness.Summary, \"在线阻断项：\"+",
			"readiness.Summary = uniqueStrings(readiness.Summary)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"shapeSheinSubmitReadinessSummary",
		})
	})

	t.Run("shared_summary_seam_owns_summary_post_shape_policy", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "shein_submit_readiness_summary.go", "shapeSheinSubmitReadinessSummary")
		callNames := readNamedFunctionCallNames(t, "shein_submit_readiness_summary.go", "shapeSheinSubmitReadinessSummary")

		assertSourceContainsAll(t, source, []string{
			"if shape.prependFirstBlocker {",
			"label := strings.TrimSpace(shape.blockingLabel)",
			"label := strings.TrimSpace(shape.warningLabel)",
			"readiness.Summary = uniqueStrings(readiness.Summary)",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"buildSheinPublishRequestForTask",
			"validateSheinOnlineAuthPreflight",
			"buildSheinReadinessGuidance",
		})
	})
}

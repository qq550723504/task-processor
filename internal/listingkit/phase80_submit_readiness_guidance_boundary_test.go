package listingkit

import "testing"

func TestSheinSubmitReadinessGuidanceBoundary(t *testing.T) {
	t.Parallel()

	t.Run("submit_readiness_builder_delegates_guidance_resolver_to_shared_seam", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "shein_submit_readiness.go", "buildSheinSubmitReadinessWithPodForAction")
		callNames := readNamedFunctionCallNames(t, "shein_submit_readiness.go", "buildSheinSubmitReadinessWithPodForAction")

		assertSourceContainsAll(t, source, []string{
			"buildSheinSubmitReadinessGuidanceResolver(pkg)",
		})
		assertSourceExcludesAll(t, source, []string{
			"guidance := buildSheinReadinessGuidance(pkg, spec.Key, spec.FieldPaths, spec.SuggestedAction, spec.WarningOnly)",
			"Reason:      cloneSheinReadinessReason(guidance.reason)",
			"RepairHints: cloneSheinRepairHints(guidance.repairHints)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildSheinSubmitReadinessGuidanceResolver",
		})
	})

	t.Run("freshness_readiness_builder_delegates_guidance_resolver_to_shared_seam", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "shein_submit_freshness.go", "buildSheinSubmitFreshnessReadiness")
		callNames := readNamedFunctionCallNames(t, "shein_submit_freshness.go", "buildSheinSubmitFreshnessReadiness")

		assertSourceContainsAll(t, source, []string{
			"buildSheinSubmitReadinessGuidanceResolver(pkg)",
		})
		assertSourceExcludesAll(t, source, []string{
			"guidance := buildSheinReadinessGuidance(pkg, spec.Key, spec.FieldPaths, spec.SuggestedAction, spec.WarningOnly)",
			"Reason:      cloneSheinReadinessReason(guidance.reason)",
			"RepairHints: cloneSheinRepairHints(guidance.repairHints)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildSheinSubmitReadinessGuidanceResolver",
		})
	})

	t.Run("shared_guidance_resolver_seam_owns_guidance_cloning_contract", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "shein_submit_readiness_guidance.go", "buildSheinSubmitReadinessGuidanceResolver")
		callNames := readNamedFunctionCallNames(t, "shein_submit_readiness_guidance.go", "buildSheinSubmitReadinessGuidanceResolver")

		assertSourceContainsAll(t, source, []string{
			"guidance := buildSheinReadinessGuidance(pkg, spec.Key, spec.FieldPaths, spec.SuggestedAction, spec.WarningOnly)",
			"Reason:      cloneSheinReadinessReason(guidance.reason)",
			"RepairHints: cloneSheinRepairHints(guidance.repairHints)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"buildSheinReadinessGuidance",
			"cloneSheinReadinessReason",
			"cloneSheinRepairHints",
		})
	})
}

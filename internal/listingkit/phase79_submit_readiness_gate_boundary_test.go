package listingkit

import "testing"

func TestSheinSubmitReadinessGateBoundary(t *testing.T) {
	t.Parallel()

	t.Run("direct_submit_flow_delegates_readiness_gating_to_shared_seam", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_direct_submission_service.go", "submitSheinTaskDirect")
		callNames := readNamedFunctionCallNames(t, "task_direct_submission_service.go", "submitSheinTaskDirect")

		assertSourceContainsAll(t, source, []string{
			"if err := validateSheinSubmitReadinessGates(ctx, task, pkg, opts.action, readiness, s.validateSheinPublishFreshness); err != nil {",
		})
		assertSourceExcludesAll(t, source, []string{
			"firstSubmitReadinessMessage(readiness)",
			"firstSubmitReadinessMessage(freshness)",
			"if s.validateSheinPublishFreshness != nil {",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"validateSheinSubmitReadinessGates",
		})
	})

	t.Run("temporal_submit_flow_delegates_readiness_gating_to_shared_seam", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_temporal_submission_adapter.go", "ValidateSheinPublishReadiness")
		callNames := readNamedFunctionCallNames(t, "task_temporal_submission_adapter.go", "ValidateSheinPublishReadiness")

		assertSourceContainsAll(t, source, []string{
			"if err := validateSheinSubmitReadinessGates(ctx, task, pkg, in.Action, readiness, s.validateSheinPublishFreshness); err != nil {",
		})
		assertSourceExcludesAll(t, source, []string{
			"firstSubmitReadinessMessage(readiness)",
			"firstSubmitReadinessMessage(freshness)",
			"if s.validateSheinPublishFreshness != nil {",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"validateSheinSubmitReadinessGates",
		})
	})

	t.Run("shared_gate_seam_owns_base_and_freshness_blocking_contract", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "submit_readiness_gate_shein.go", "validateSheinSubmitReadinessGates")
		callNames := readNamedFunctionCallNames(t, "submit_readiness_gate_shein.go", "validateSheinSubmitReadinessGates")

		assertSourceContainsAll(t, source, []string{
			"if readiness == nil || !readiness.Ready {",
			"return fmt.Errorf(\"%w: %s\", ErrSubmitBlocked, firstSubmitReadinessMessage(readiness))",
			"freshness, err := validateFreshness(ctx, task, pkg, action)",
			"return fmt.Errorf(\"%w: %s\", ErrSubmitBlocked, firstSubmitReadinessMessage(freshness))",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"firstSubmitReadinessMessage",
		})
	})
}

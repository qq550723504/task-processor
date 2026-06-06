package listingkit

import "testing"

func TestGenerationNavigationDescriptorDispatchPlanDelegationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("descriptor_dispatch_plan_delegation_home_owns_only_dispatch_plan_delegation", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_clone_shape_pairing.go", "applyGenerationNavigationDescriptorCloneShapePairing")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_clone_shape_pairing.go", "applyGenerationNavigationDescriptorCloneShapePairing")

		assertSourceContainsAll(t, source, []string{
			"cloned.DispatchPlan = cloneGenerationNavigationDispatchPlan(descriptor.DispatchPlan)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneGenerationNavigationFollowUpRead(",
			"cloneGenerationQueueQuery(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationNavigationDispatchPlan",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationFollowUpRead",
			"cloneGenerationQueueQuery",
		})
	})

	t.Run("descriptor_clone_shape_home_keeps_dispatch_plan_out_of_aggregate_run", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "run")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_target_identity.go", "run")

		assertSourceContainsAll(t, source, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing(descriptor, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.DispatchPlan = cloneGenerationNavigationDispatchPlan(descriptor.DispatchPlan)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationDispatchPlan",
		})
	})
}

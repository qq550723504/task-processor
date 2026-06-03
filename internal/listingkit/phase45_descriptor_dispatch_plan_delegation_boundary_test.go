package listingkit

import "testing"

func TestGenerationNavigationDescriptorDispatchPlanDelegationBoundary(t *testing.T) {
	t.Parallel()

	t.Run("descriptor_dispatch_plan_delegation_home_owns_only_dispatch_plan_delegation", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_dispatch_plan_delegation.go", "applyGenerationNavigationDescriptorDispatchPlanCloneDelegation")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_dispatch_plan_delegation.go", "applyGenerationNavigationDescriptorDispatchPlanCloneDelegation")

		assertSourceContainsAll(t, source, []string{
			"cloned.DispatchPlan = cloneGenerationNavigationDispatchPlan(descriptor.DispatchPlan)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloneGenerationConditionalState(",
			"cloneGenerationNavigationFollowUpRead(",
			"cloneGenerationQueueQuery(",
			"append([]string(nil), descriptor.Invalidates...)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationNavigationDispatchPlan",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationFollowUpRead",
			"cloneGenerationQueueQuery",
		})
	})

	t.Run("descriptor_residual_shape_home_routes_dispatch_plan_through_local_home", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_descriptor_residual_shape.go", "applyGenerationNavigationDescriptorResidualCloneShape")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_residual_shape.go", "applyGenerationNavigationDescriptorResidualCloneShape")

		assertSourceContainsAll(t, source, []string{
			"applyGenerationNavigationDescriptorResidualClonePairing(descriptor, cloned)",
			"applyGenerationNavigationDescriptorDispatchPlanCloneDelegation(descriptor, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.DispatchPlan = cloneGenerationNavigationDispatchPlan(descriptor.DispatchPlan)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorResidualClonePairing",
			"applyGenerationNavigationDescriptorDispatchPlanCloneDelegation",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationDispatchPlan",
		})
	})
}

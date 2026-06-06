package listingkit

import "testing"

func TestGenerationNavigationDispatchPlanStepCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("dispatch_plan_step_clone_home_owns_only_step_specific_shaping", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "generation_navigation_dispatch_plan_step_clone.go", "cloneGenerationNavigationDispatchPlanStep")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_dispatch_plan_step_clone.go", "cloneGenerationNavigationDispatchPlanStep")

		assertSourceContainsAll(t, source, []string{
			"Kind:               step.Kind,",
			"ResponseMode:       step.ResponseMode,",
			"CachePreference:    step.CachePreference,",
			"RequiresRevalidate: step.RequiresRevalidate,",
			"Query:              cloneGenerationQueueQuery(step.Query),",
		})
		assertSourceExcludesAll(t, source, []string{
			"func cloneGenerationNavigationDispatchPlan(",
			"func cloneGenerationNavigationDescriptor(",
			"cloneGenerationConditionalState(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationQueueQuery",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDescriptor",
		})
	})

	t.Run("dispatch_plan_clone_shape_home_routes_through_step_clone_home", func(t *testing.T) {
		t.Parallel()

		source := readTaskGenerationSourceFile(t, "generation_navigation_target_identity.go")

		assertSourceContainsAll(t, source, []string{
			"cloned.Steps = make([]GenerationNavigationDispatchStep, 0, len(plan.Steps))",
			"cloned.Steps = append(cloned.Steps, cloneGenerationNavigationDispatchPlanStep(step))",
		})
		assertSourceExcludesAll(t, source, []string{
			"Kind:               step.Kind,",
			"ResponseMode:       step.ResponseMode,",
			"CachePreference:    step.CachePreference,",
			"RequiresRevalidate: step.RequiresRevalidate,",
			"Query:              cloneGenerationQueueQuery(step.Query),",
		})
	})
}

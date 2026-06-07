package listingkit

import "testing"

func TestGenerationNavigationDispatchPlanCloneAggregateBoundary(t *testing.T) {
	t.Parallel()

	t.Run("aggregate_dispatch_plan_clone_owner_delegates_nested_step_query_clone_shaping", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "generation_navigation_target_identity.go")
		source := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "cloneGenerationNavigationDispatchPlan")
		assertSourceContainsAll(t, fileSource, []string{
			"func cloneGenerationNavigationDispatchPlan(",
			"func cloneGenerationNavigationDispatchPlanStep(",
			"if plan == nil {",
			"cloned := *plan",
			"if len(plan.Steps) > 0 {",
			"cloned.Steps = make([]GenerationNavigationDispatchStep, 0, len(plan.Steps))",
			"cloned.Steps = append(cloned.Steps, cloneGenerationNavigationDispatchPlanStep(step))",
			"return &cloned",
		})
		assertSourceContainsAll(t, source, []string{
			"cloned := *plan",
			"if len(plan.Steps) > 0 {",
			"cloned.Steps = make([]GenerationNavigationDispatchStep, 0, len(plan.Steps))",
			"cloned.Steps = append(cloned.Steps, cloneGenerationNavigationDispatchPlanStep(step))",
		})
		assertSourceOccurrenceCount(t, fileSource, "cloned.Steps = append(cloned.Steps, cloneGenerationNavigationDispatchPlanStep(step))", 1)
		assertSourceExcludesAll(t, fileSource, []string{
			"buildTaskGenerationNavigationDispatchPlanPhase(",
		})
	})
}

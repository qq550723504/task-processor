package listingkit

import "testing"

func TestGenerationNavigationDispatchPlanCloneAggregateBoundary(t *testing.T) {
	t.Parallel()

	t.Run("aggregate_dispatch_plan_clone_shape_owner_delegates_nested_step_query_clone_shaping", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "generation_navigation_dispatch_plan_clone_shape.go")
		buildSource := readNamedFunctionSource(t, "generation_navigation_dispatch_plan_clone_shape.go", "buildGenerationNavigationDispatchPlanCloneShapePhase")
		source := readNamedFunctionSource(t, "generation_navigation_dispatch_plan_clone_shape.go", "run")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_dispatch_plan_clone_shape.go", "run")

		assertSourceContainsAll(t, fileSource, []string{
			"type generationNavigationDispatchPlanCloneShapePhase struct{}",
			"func buildGenerationNavigationDispatchPlanCloneShapePhase()",
			"func (p *generationNavigationDispatchPlanCloneShapePhase) run(",
		})
		assertSourceContainsAll(t, buildSource, []string{
			"return &generationNavigationDispatchPlanCloneShapePhase{}",
		})
		assertSourceContainsAll(t, source, []string{
			"if plan == nil || cloned == nil {",
			"if len(plan.Steps) == 0 {",
			"cloned.Steps = make([]GenerationNavigationDispatchStep, 0, len(plan.Steps))",
			"Kind:               step.Kind,",
			"ResponseMode:       step.ResponseMode,",
			"CachePreference:    step.CachePreference,",
			"RequiresRevalidate: step.RequiresRevalidate,",
			"Query:              cloneGenerationQueueQuery(step.Query),",
		})
		assertSourceExcludesAll(t, source, []string{
			"func cloneGenerationNavigationDispatchPlan(",
			"cloneGenerationNavigationDescriptor(",
			"buildTaskGenerationNavigationDispatchPlanPhase(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneGenerationQueueQuery",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationNavigationDescriptor",
			"buildTaskGenerationNavigationDispatchPlanPhase",
		})
	})
}

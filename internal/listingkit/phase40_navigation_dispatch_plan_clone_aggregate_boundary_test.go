package listingkit

import "testing"

func TestGenerationNavigationDispatchPlanCloneAggregateBoundary(t *testing.T) {
	t.Parallel()

	t.Run("aggregate_dispatch_plan_clone_shape_owner_delegates_nested_step_query_clone_shaping", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "generation_navigation_target_identity.go")
		buildSource := readNamedFunctionSource(t, "generation_navigation_target_identity.go", "buildGenerationNavigationDispatchPlanCloneShapePhase")
		assertSourceContainsAll(t, fileSource, []string{
			"type generationNavigationDispatchPlanCloneShapePhase struct{}",
			"func buildGenerationNavigationDispatchPlanCloneShapePhase()",
			"func (p *generationNavigationDispatchPlanCloneShapePhase) run(",
			"func cloneGenerationNavigationDispatchPlanStep(",
			"if plan == nil || cloned == nil {",
			"if len(plan.Steps) == 0 {",
			"cloned.Steps = make([]GenerationNavigationDispatchStep, 0, len(plan.Steps))",
			"cloned.Steps = append(cloned.Steps, cloneGenerationNavigationDispatchPlanStep(step))",
		})
		assertSourceContainsAll(t, buildSource, []string{
			"return &generationNavigationDispatchPlanCloneShapePhase{}",
		})
		assertSourceOccurrenceCount(t, fileSource, "cloned.Steps = append(cloned.Steps, cloneGenerationNavigationDispatchPlanStep(step))", 1)
		assertSourceExcludesAll(t, fileSource, []string{
			"buildTaskGenerationNavigationDispatchPlanPhase(",
		})
	})
}

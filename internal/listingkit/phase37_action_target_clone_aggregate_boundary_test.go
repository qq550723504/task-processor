package listingkit

import "testing"

func TestTaskGenerationActionTargetCloneAggregateBoundary(t *testing.T) {
	t.Parallel()

	t.Run("aggregate_clone_shape_owner_delegates_all_nested_clone_shaping", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "task_generation_action_target_clone_shape.go")
		buildSource := readNamedFunctionSource(t, "task_generation_action_target_clone_shape.go", "buildTaskGenerationActionTargetCloneShapePhase")
		source := readNamedFunctionSource(t, "task_generation_action_target_clone_shape.go", "run")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_target_clone_shape.go", "run")

		assertSourceContainsAll(t, fileSource, []string{
			"type taskGenerationActionTargetCloneShapePhase struct{}",
			"func buildTaskGenerationActionTargetCloneShapePhase()",
			"func (p *taskGenerationActionTargetCloneShapePhase) run(",
		})
		assertSourceContainsAll(t, buildSource, []string{
			"return &taskGenerationActionTargetCloneShapePhase{}",
		})
		assertSourceContainsAll(t, source, []string{
			"if target == nil || cloned == nil {",
			"cloned.Filters = cloneAssetGenerationFilters(target.Filters)",
			"cloned.QueueQuery = cloneGenerationQueueQuery(target.QueueQuery)",
			"cloned.RetryRequest = cloneRetryGenerationTasksRequest(target.RetryRequest)",
			"cloned.ExpectedImpact = cloneAssetGenerationActionImpact(target.ExpectedImpact)",
			"cloned.NavigationTarget = cloneGenerationReviewNavigationTarget(target.NavigationTarget)",
		})
		assertSourceExcludesAll(t, source, []string{
			"func cloneGenerationQueueQuery(",
			"func cloneRetryGenerationTasksRequest(",
			"func cloneAssetGenerationActionTarget(",
			"resolveAssetGenerationActionTarget(",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"cloneAssetGenerationFilters",
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
			"cloneAssetGenerationActionImpact",
			"cloneGenerationReviewNavigationTarget",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"resolveAssetGenerationActionTarget",
		})
	})
}

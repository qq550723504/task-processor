package listingkit

import "testing"

func TestActionTargetImpactCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("action_target_impact_clone_home_owns_top_level_copy_only", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_action_target_clone.go", "cloneAssetGenerationActionImpact")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_target_clone.go", "cloneAssetGenerationActionImpact")

		assertSourceContainsAll(t, source, []string{
			"cloned := *impact",
			"applyAssetGenerationActionImpactCloneShape(impact, &cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.Platforms = append([]string(nil), impact.Platforms...)",
			"cloned.QualityGrades = append([]string(nil), impact.QualityGrades...)",
			"cloned.States = append([]string(nil), impact.States...)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationActionImpactCloneShape",
		})
	})

	t.Run("action_target_impact_shape_home_owns_slice_clone_shaping", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_action_impact_clone_shape.go", "applyAssetGenerationActionImpactCloneShape")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_impact_clone_shape.go", "applyAssetGenerationActionImpactCloneShape")

		assertSourceContainsAll(t, source, []string{
			"applyAssetGenerationActionImpactSliceClone(impact, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.Platforms = append([]string(nil), impact.Platforms...)",
			"cloned.QualityGrades = append([]string(nil), impact.QualityGrades...)",
			"cloned.States = append([]string(nil), impact.States...)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationActionImpactSliceClone",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationQueueQuery",
			"cloneRetryGenerationTasksRequest",
			"cloneAssetGenerationActionTarget",
		})
	})
}

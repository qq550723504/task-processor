package listingkit

import "testing"

func TestActionTargetImpactFinalSliceBoundary(t *testing.T) {
	t.Parallel()

	t.Run("action_target_impact_platforms_clone_home_owns_only_platforms_slice", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_action_target_clone.go", "applyAssetGenerationActionImpactPlatformsClone")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_target_clone.go", "applyAssetGenerationActionImpactPlatformsClone")

		assertSourceContainsAll(t, source, []string{
			"cloned.Platforms = append([]string(nil), impact.Platforms...)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.QualityGrades = append([]string(nil), impact.QualityGrades...)",
			"cloned.States = append([]string(nil), impact.States...)",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyAssetGenerationActionImpactQualityGradesClone",
			"applyAssetGenerationActionImpactStatesClone",
			"applyAssetGenerationActionImpactSliceClone",
		})
	})

	t.Run("action_target_impact_quality_grades_clone_home_owns_only_quality_grades_slice", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_action_target_clone.go", "applyAssetGenerationActionImpactQualityGradesClone")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_target_clone.go", "applyAssetGenerationActionImpactQualityGradesClone")

		assertSourceContainsAll(t, source, []string{
			"cloned.QualityGrades = append([]string(nil), impact.QualityGrades...)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.Platforms = append([]string(nil), impact.Platforms...)",
			"cloned.States = append([]string(nil), impact.States...)",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyAssetGenerationActionImpactPlatformsClone",
			"applyAssetGenerationActionImpactStatesClone",
			"applyAssetGenerationActionImpactSliceClone",
		})
	})

	t.Run("action_target_impact_states_clone_home_owns_only_states_slice", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_action_target_clone.go", "applyAssetGenerationActionImpactStatesClone")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_target_clone.go", "applyAssetGenerationActionImpactStatesClone")

		assertSourceContainsAll(t, source, []string{
			"cloned.States = append([]string(nil), impact.States...)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.Platforms = append([]string(nil), impact.Platforms...)",
			"cloned.QualityGrades = append([]string(nil), impact.QualityGrades...)",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"applyAssetGenerationActionImpactPlatformsClone",
			"applyAssetGenerationActionImpactQualityGradesClone",
			"applyAssetGenerationActionImpactSliceClone",
		})
	})
}

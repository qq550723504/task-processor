package listingkit

import "testing"

func TestActionTargetImpactSliceCloneBoundary(t *testing.T) {
	t.Parallel()

	t.Run("action_target_impact_clone_home_owns_platform_quality_state_clones", func(t *testing.T) {
		t.Parallel()

		source := readNamedFunctionSource(t, "task_generation_action_target_clone.go", "cloneAssetGenerationActionImpact")
		callNames := readNamedFunctionCallNames(t, "task_generation_action_target_clone.go", "cloneAssetGenerationActionImpact")

		assertSourceContainsAll(t, source, []string{
			"applyAssetGenerationActionImpactPlatformsClone(impact, &cloned)",
			"applyAssetGenerationActionImpactQualityGradesClone(impact, &cloned)",
			"applyAssetGenerationActionImpactStatesClone(impact, &cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"cloned.Platforms = append([]string(nil), impact.Platforms...)",
			"cloned.QualityGrades = append([]string(nil), impact.QualityGrades...)",
			"cloned.States = append([]string(nil), impact.States...)",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyAssetGenerationActionImpactPlatformsClone",
			"applyAssetGenerationActionImpactQualityGradesClone",
			"applyAssetGenerationActionImpactStatesClone",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneAssetGenerationActionImpact",
			"cloneGenerationQueueQuery",
		})
	})
}

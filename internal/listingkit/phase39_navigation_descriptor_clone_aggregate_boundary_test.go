package listingkit

import "testing"

func TestGenerationNavigationDescriptorCloneAggregateBoundary(t *testing.T) {
	t.Parallel()

	t.Run("aggregate_descriptor_clone_shape_owner_delegates_nested_clone_shaping", func(t *testing.T) {
		t.Parallel()

		fileSource := readTaskGenerationSourceFile(t, "generation_navigation_descriptor_clone_shape.go")
		buildSource := readNamedFunctionSource(t, "generation_navigation_descriptor_clone_shape.go", "buildGenerationNavigationDescriptorCloneShapePhase")
		source := readNamedFunctionSource(t, "generation_navigation_descriptor_clone_shape.go", "run")
		callNames := readNamedFunctionCallNames(t, "generation_navigation_descriptor_clone_shape.go", "run")

		assertSourceContainsAll(t, fileSource, []string{
			"type generationNavigationDescriptorCloneShapePhase struct{}",
			"func buildGenerationNavigationDescriptorCloneShapePhase()",
			"func (p *generationNavigationDescriptorCloneShapePhase) run(",
		})
		assertSourceContainsAll(t, buildSource, []string{
			"return &generationNavigationDescriptorCloneShapePhase{}",
		})
		assertSourceContainsAll(t, source, []string{
			"if descriptor == nil || cloned == nil {",
			"applyGenerationNavigationDescriptorCloneShapePairing(descriptor, cloned)",
		})
		assertSourceExcludesAll(t, source, []string{
			"func cloneGenerationNavigationDescriptor(",
			"buildGenerationReviewNavigationTarget(",
			"cloneGenerationReviewNavigationTarget(",
			"cloned.Conditional = cloneGenerationConditionalState(descriptor.Conditional)",
			"cloned.DispatchPlan = cloneGenerationNavigationDispatchPlan(descriptor.DispatchPlan)",
			"cloned.Invalidates = append([]string(nil), descriptor.Invalidates...)",
			"cloned.FollowUpReads = make([]GenerationNavigationFollowUpRead, 0, len(descriptor.FollowUpReads))",
		})
		assertFunctionCallsContainAll(t, callNames, []string{
			"applyGenerationNavigationDescriptorCloneShapePairing",
		})
		assertFunctionCallsExcludeAll(t, callNames, []string{
			"cloneGenerationConditionalState",
			"cloneGenerationNavigationDispatchPlan",
			"cloneGenerationQueueQuery",
			"buildGenerationReviewNavigationTarget",
			"cloneGenerationReviewNavigationTarget",
		})
	})
}

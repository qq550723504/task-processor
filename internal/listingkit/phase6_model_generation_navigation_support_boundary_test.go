package listingkit

import "testing"

func TestModelGenerationNavigationSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "model_generation_navigation.go")
	assertSourceContainsAll(t, rootSource, []string{
		"type GenerationReviewNavigationDelta struct {",
		"type GenerationReviewNavigationTarget struct {",
		"type GenerationNavigationDescriptor struct {",
		"type GenerationNavigationDispatchPlan struct {",
		"type GenerationNavigationDispatchStep struct {",
		"type GenerationNavigationFollowUpRead struct {",
		"type GenerationReviewNavigationDispatchRequest struct {",
		"type GenerationReviewNavigationDispatchResponse struct {",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"type GenerationNavigationDispatchExecution struct {",
		"type GenerationNavigationDispatchExecutionStep struct {",
		"type GenerationReviewPanelUpdate struct {",
		"type GenerationPanelResourceDescriptor struct {",
		"type GenerationNavigationDispatchResolution struct {",
	})

	supportSource := readTaskGenerationSourceFile(t, "model_generation_navigation_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"type GenerationNavigationDispatchExecution struct {",
		"type GenerationNavigationDispatchExecutionStep struct {",
		"type GenerationReviewPanelUpdate struct {",
		"type GenerationPanelResourceDescriptor struct {",
		"type GenerationNavigationDispatchResolution struct {",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"type GenerationReviewNavigationTarget struct {",
		"type GenerationNavigationDescriptor struct {",
		"type GenerationReviewNavigationDispatchResponse struct {",
	})
}

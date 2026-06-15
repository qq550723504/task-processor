package listingkit

import "testing"

func TestModelRequestSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "model_request.go")
	assertSourceContainsAll(t, rootSource, []string{
		"type GenerateRequest struct {",
		"type WarmSDSBaselineRequest struct {",
		"type GenerateOptions struct {",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"type SheinStudioOptions struct {",
		"type StudioDesignRequest struct {",
		"type SDSSyncOptions struct {",
		"type SubmitTaskRequest struct {",
		"type SheinSettings struct {",
		"type SheinCategorySearchResult struct {",
	})

	supportSource := readTaskGenerationSourceFile(t, "model_request_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"type modelRequestSupportBoundary struct{}",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"type SheinStudioOptions struct {",
		"type StudioDesignRequest struct {",
		"type SDSSyncOptions struct {",
		"type SubmitTaskRequest struct {",
		"type SheinSettings struct {",
		"type SheinCategorySearchResult struct {",
		"type GenerateRequest struct {",
		"type WarmSDSBaselineRequest struct {",
		"type GenerateOptions struct {",
	})

	studioSource := readTaskGenerationSourceFile(t, "model_request_studio_support.go")
	assertSourceContainsAll(t, studioSource, []string{
		"type SheinStudioOptions struct {",
		"type StudioDesignRequest struct {",
		"type SDSSyncOptions struct {",
	})
	assertSourceExcludesAll(t, studioSource, []string{
		"type SubmitTaskRequest struct {",
		"type SheinSettings struct {",
		"type SheinCategorySearchResult struct {",
	})

	submitSource := readTaskGenerationSourceFile(t, "model_request_submit_support.go")
	assertSourceContainsAll(t, submitSource, []string{
		"type SubmitTaskRequest struct {",
		"type SheinSettings struct {",
		"type SheinCategorySearchResult struct {",
	})
	assertSourceExcludesAll(t, submitSource, []string{
		"type SheinStudioOptions struct {",
		"type StudioDesignRequest struct {",
		"type SDSSyncOptions struct {",
	})
}

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
		"type SheinStudioOptions struct {",
		"type StudioDesignRequest struct {",
		"type SDSSyncOptions struct {",
		"type SubmitTaskRequest struct {",
		"type SheinSettings struct {",
		"type SheinCategorySearchResult struct {",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"type GenerateRequest struct {",
		"type WarmSDSBaselineRequest struct {",
		"type GenerateOptions struct {",
	})
}

package listingkit

import "testing"

func TestStudioSessionModelSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "studio_session_model.go")
	assertSourceContainsAll(t, rootSource, []string{
		"type SheinStudioSessionStatus string",
		"type SheinStudioCreatedTask struct {",
		"type SheinStudioGenerationJob struct {",
		"type SheinStudioSession struct {",
		"type SheinStudioDesign struct {",
		"type SheinStudioSessionDetail struct {",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"type SheinStudioSelection struct {",
		"type SheinStudioProductImagePrompt struct {",
		"type SheinStudioGroupedSelection struct {",
		"type UpsertStudioBatchRequest struct {",
		"type SheinStudioBatchListItem struct {",
		"type StudioBatchListResponse struct {",
	})

	supportSource := readTaskGenerationSourceFile(t, "studio_session_model_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"type SheinStudioSelection struct {",
		"type SheinStudioProductImagePrompt struct {",
		"type SheinStudioGroupedSelection struct {",
		"type UpsertStudioBatchRequest struct {",
		"type SheinStudioBatchListItem struct {",
		"type StudioBatchListResponse struct {",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"type SheinStudioSession struct {",
		"type SheinStudioDesign struct {",
		"type SheinStudioSessionDetail struct {",
	})
}

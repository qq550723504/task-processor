package listingkit

import "testing"

func TestSheinFinalDraftSupportBoundary(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "shein_final_draft.go")

	adminSource := readTaskGenerationSourceFile(t, "shein_admin_service_support.go")
	submitNormalizeSource := readTaskGenerationSourceFile(t, "task_submission_execution_normalize.go")
	assertSourceContainsAll(t, adminSource, []string{
		"sheinpub.ApplyFinalImageDraft(pkg)",
	})
	assertSourceContainsAll(t, submitNormalizeSource, []string{
		"sheinpub.ApplyFinalImageDraft(pkg)",
	})

	publishingSource := readTaskGenerationSourceFile(t, "../publishing/shein/final_draft_images.go")
	assertSourceContainsAll(t, publishingSource, []string{
		"func ApplyFinalImageDraft(pkg *Package) {",
		"func EnsureFinalDraftSKCImages(pkg *Package, main string, order []string, deleted map[string]struct{}) {",
		"func EnsureFinalPreviewSKCImages(pkg *Package) {",
		"func OrderFinalDraftImages(existing []string, order []string, deleted map[string]struct{}) []string {",
		"func FinalDraftFallbackImages(pkg *Package, main string, deleted map[string]struct{}) []string {",
		"func RequestDraftSKCByIndexOrCode(draft *RequestDraft, index int, supplierCode string) *SKCRequestDraft {",
		"func ReorderFinalDraftProductImages(info *sheinproduct.ImageInfo, order []string, main string, deleted map[string]struct{}, roles map[string]string) {",
		"func NormalizeImageRoleOverrides(input map[string]string) map[string]string {",
	})
	assertSourceExcludesAll(t, adminSource+submitNormalizeSource, []string{
		"for _, image := range pkg.FinalSubmissionDraft.DeletedImageURLs",
		"pkg.DraftPayload.ImageInfo.Gallery = images",
		"reorderSheinProductImages(",
		"func orderSheinImages(existing []string, order []string, deleted map[string]struct{}) []string {",
		"func sheinFinalDraftFallbackImages(pkg *sheinpub.Package, main string, deleted map[string]struct{}) []string {",
		"func sheinRequestDraftSKCByIndexOrCode(draft *sheinpub.RequestDraft, index int, supplierCode string) *sheinpub.SKCRequestDraft {",
		"func reorderSheinProductImages(info *sheinproduct.ImageInfo, order []string, main string, deleted map[string]struct{}, roles map[string]string) {",
		"func normalizeImageRoleOverrides(input map[string]string) map[string]string {",
	})
}

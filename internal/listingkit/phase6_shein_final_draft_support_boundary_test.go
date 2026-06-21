package listingkit

import "testing"

func TestSheinFinalDraftSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "shein_final_draft.go")
	assertSourceContainsAll(t, rootSource, []string{
		"func applySheinFinalImageDraft(pkg *sheinpub.Package) {",
		"func ensureSheinFinalDraftSKCImages(pkg *sheinpub.Package, main string, order []string, deleted map[string]struct{}) {",
		"func ensureSheinFinalPreviewSKCImages(pkg *sheinpub.Package) {",
		"sheinpub.ApplyFinalImageDraft(pkg)",
		"sheinpub.EnsureFinalDraftSKCImages(pkg, main, order, deleted)",
		"sheinpub.EnsureFinalPreviewSKCImages(pkg)",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"for _, image := range pkg.FinalSubmissionDraft.DeletedImageURLs",
		"pkg.DraftPayload.ImageInfo.Gallery = images",
		"reorderSheinProductImages(",
		"func orderSheinImages(existing []string, order []string, deleted map[string]struct{}) []string {",
		"func sheinFinalDraftFallbackImages(pkg *sheinpub.Package, main string, deleted map[string]struct{}) []string {",
		"func sheinRequestDraftSKCByIndexOrCode(draft *sheinpub.RequestDraft, index int, supplierCode string) *sheinpub.SKCRequestDraft {",
		"func reorderSheinProductImages(info *sheinproduct.ImageInfo, order []string, main string, deleted map[string]struct{}, roles map[string]string) {",
		"func normalizeImageRoleOverrides(input map[string]string) map[string]string {",
	})

	assertFileAbsent(t, "shein_final_draft_support.go")

	publishingSource := readTaskGenerationSourceFile(t, "../publishing/shein/final_draft_images.go")
	assertSourceContainsAll(t, publishingSource, []string{
		"func OrderFinalDraftImages(existing []string, order []string, deleted map[string]struct{}) []string {",
		"func FinalDraftFallbackImages(pkg *Package, main string, deleted map[string]struct{}) []string {",
		"func RequestDraftSKCByIndexOrCode(draft *RequestDraft, index int, supplierCode string) *SKCRequestDraft {",
		"func ReorderFinalDraftProductImages(info *sheinproduct.ImageInfo, order []string, main string, deleted map[string]struct{}, roles map[string]string) {",
		"func NormalizeImageRoleOverrides(input map[string]string) map[string]string {",
	})
}

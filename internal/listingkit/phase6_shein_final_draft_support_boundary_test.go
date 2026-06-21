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

	supportSource := readTaskGenerationSourceFile(t, "shein_final_draft_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"func orderSheinImages(existing []string, order []string, deleted map[string]struct{}) []string {",
		"func sheinFinalDraftFallbackImages(pkg *sheinpub.Package, main string, deleted map[string]struct{}) []string {",
		"func sheinRequestDraftSKCByIndexOrCode(draft *sheinpub.RequestDraft, index int, supplierCode string) *sheinpub.SKCRequestDraft {",
		"func reorderSheinProductImages(info *sheinproduct.ImageInfo, order []string, main string, deleted map[string]struct{}, roles map[string]string) {",
		"func normalizeImageRoleOverrides(input map[string]string) map[string]string {",
		"return sheinpub.OrderFinalDraftImages(existing, order, deleted)",
		"return sheinpub.FinalDraftFallbackImages(pkg, main, deleted)",
		"return sheinpub.RequestDraftSKCByIndexOrCode(draft, index, supplierCode)",
		"sheinpub.ReorderFinalDraftProductImages(info, order, main, deleted, roles)",
		"return sheinpub.NormalizeImageRoleOverrides(input)",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"for _, image := range pkg.PreviewPayload.ImageInfo.ImageInfoList",
		"switch strings.ToLower(strings.TrimSpace(role))",
		"func applySheinFinalImageDraft(pkg *sheinpub.Package) {",
		"func ensureSheinFinalDraftSKCImages(pkg *sheinpub.Package, main string, order []string, deleted map[string]struct{}) {",
		"func ensureSheinFinalPreviewSKCImages(pkg *sheinpub.Package) {",
	})
}

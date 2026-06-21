package listingkit

import "testing"

func TestSDSSheinImagesSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "sds_shein_images.go")
	assertSourceContainsAll(t, rootSource, []string{
		"func applySelectedSDSImagesToShein(pkg *sheinpub.Package, req *GenerateRequest, sourceImages []string) bool {",
		"func applySDSOfficialImagesToShein(pkg *sheinpub.Package, _ *GenerateRequest, summary *SDSSyncSummary, options *SDSSyncOptions) bool {",
		"func applySDSTemplateImagesToShein(pkg *sheinpub.Package, summary *SDSSyncSummary, sourceImages []string, options ...*SDSSyncOptions) {",
		"func applySDSTemplateImagesToSheinWithResult(pkg *sheinpub.Package, summary *SDSSyncSummary, sourceImages []string, options *SDSSyncOptions) bool {",
		"func applySDSVariantTemplateImagesToShein(pkg *sheinpub.Package, summary *SDSSyncSummary, sourceImages []string, options *SDSSyncOptions) bool {",
		"func hasSDSVariantOptionMockups(options *SDSSyncOptions) bool {",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"func resolveSDSImagesForSKC(pkg *sheinpub.Package, index int, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {",
		"func sdsSKUCandidatesFromRequestSKC(skc *sheinpub.SKCRequestDraft) []string {",
		"func imageSetFromSDSMockups(mockups []string, sourceImages []string) *common.ImageSet {",
		"func normalizeSelectedSDSImages(input []SheinStudioSelectedSDSImage) []SheinStudioSelectedSDSImage {",
		"func normalizeSDSColorKey(value string) string {",
	})

	supportSource := readTaskGenerationSourceFile(t, "sds_shein_images_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"func resolveSDSImagesForSKC(pkg *sheinpub.Package, index int, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {",
		"return sheinpub.ResolveSDSImagesForSKC(pkg, index, bySKU, byColor)",
		"func imageSetFromSDSMockups(mockups []string, sourceImages []string) *common.ImageSet {",
		"func normalizeSelectedSDSImages(input []SheinStudioSelectedSDSImage) []SheinStudioSelectedSDSImage {",
		"func normalizeSDSColorKey(value string) string {",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"func applySelectedSDSImagesToShein(pkg *sheinpub.Package, req *GenerateRequest, sourceImages []string) bool {",
		"func applySDSOfficialImagesToShein(pkg *sheinpub.Package, _ *GenerateRequest, summary *SDSSyncSummary, options *SDSSyncOptions) bool {",
		"func applySDSTemplateImagesToSheinWithResult(pkg *sheinpub.Package, summary *SDSSyncSummary, sourceImages []string, options *SDSSyncOptions) bool {",
		"func applySDSVariantTemplateImagesToShein(pkg *sheinpub.Package, summary *SDSSyncSummary, sourceImages []string, options *SDSSyncOptions) bool {",
		"func sdsSKUCandidatesFromRequestSKC(skc *sheinpub.SKCRequestDraft) []string {",
	})
}

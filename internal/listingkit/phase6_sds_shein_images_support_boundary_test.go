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
		"func imageSetFromSDSVariantOption(item SDSSyncVariantOption, sourceImages []string) *common.ImageSet {",
		"func imageSetFromSelectedSDSImages(items []SheinStudioSelectedSDSImage, sourceImages []string) *common.ImageSet {",
		"func normalizeSelectedSDSImages(input []SheinStudioSelectedSDSImage) []SheinStudioSelectedSDSImage {",
		"sheinpub.ResolveSDSImagesForSKC(pkg, skcIndex, bySKU, byColor)",
		"sheinpub.ImageSetFromSDSMockups(summary.MockupImageURLs, sourceImages)",
		"sheinpub.NormalizeSDSImageKey(summary.VariantColor)",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"func resolveSDSImagesForSKC(pkg *sheinpub.Package, index int, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {",
		"func sdsSKUCandidatesFromRequestSKC(skc *sheinpub.SKCRequestDraft) []string {",
		"func imageSetFromSDSMockups(mockups []string, sourceImages []string) *common.ImageSet {",
		"func normalizeSDSColorKey(value string) string {",
	})

	assertFileAbsent(t, "sds_shein_images_support.go")
	assertSourceExcludesAll(t, rootSource, []string{
		"func registerSDSVariantImageSet(",
		"func firstSDSImageSet(",
		"func resolveSDSImagesForSKC(",
		"func resolveSDSImagesForSKU(",
		"func sourceSDSSKUFromSupplierSKU(",
		"func imageSetFromSDSMockups(",
		"func mergeImageSet(",
		"func normalizeSDSColorKey(",
	})

	publishingSource := readTaskGenerationSourceFile(t, "../publishing/shein/sds_images.go")
	assertSourceContainsAll(t, publishingSource, []string{
		"func RegisterSDSVariantImageSet(bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet, sku string, color string, images *common.ImageSet, overwrite bool) {",
		"func FirstSDSImageSet(values map[string]*common.ImageSet) *common.ImageSet {",
		"func ResolveSDSImagesForSKC(pkg *Package, index int, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {",
		"func ResolveSDSImagesForSKU(sku *SKUDraft, bySKU map[string]*common.ImageSet, byColor map[string]*common.ImageSet) *common.ImageSet {",
		"func SourceSDSSKUFromSupplierSKU(value string) string {",
		"func ImageSetFromSDSMockups(mockups []string, sourceImages []string) *common.ImageSet {",
		"func MergeSDSImageSet(existing *common.ImageSet, next *common.ImageSet) *common.ImageSet {",
		"func NormalizeSDSImageKey(value string) string {",
	})
}

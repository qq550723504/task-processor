package listingkit

import "testing"

func TestWorkflowStudioSDSMetadataSupportBoundary(t *testing.T) {
	t.Parallel()

	rootSource := readTaskGenerationSourceFile(t, "workflow_studio_sds_metadata.go")
	assertSourceContainsAll(t, rootSource, []string{
		"func studioCategoryPath(sds *SDSSyncOptions) []string {",
		"func studioAttributes(sds *SDSSyncOptions, trace canonical.FieldTrace) map[string]canonical.Attribute {",
		"func studioSpecifications(sds *SDSSyncOptions) *canonical.ProductSpecs {",
		"func studioVariants(sds *SDSSyncOptions, images []canonical.Image, trace canonical.FieldTrace) []canonical.Variant {",
		"func studioSellingPoints(sds *SDSSyncOptions) []string {",
	})
	assertSourceExcludesAll(t, rootSource, []string{
		"func studioStyleName(sds *SDSSyncOptions) string {",
		"func buildStudioVariantSKU(baseSKU, styleID, variantDiscriminator string, requireVariantDiscriminator bool, seen map[string]int) string {",
		"func studioVariantDiscriminator(item SDSSyncVariantOption, index int) string {",
		"func appendNonEmpty(values []string, candidates ...string) []string {",
	})

	supportSource := readTaskGenerationSourceFile(t, "workflow_studio_sds_metadata_support.go")
	assertSourceContainsAll(t, supportSource, []string{
		"func studioStyleName(sds *SDSSyncOptions) string {",
		"func buildStudioVariantSKU(baseSKU, styleID, variantDiscriminator string, requireVariantDiscriminator bool, seen map[string]int) string {",
		"func studioVariantDiscriminator(item SDSSyncVariantOption, index int) string {",
		"func appendNonEmpty(values []string, candidates ...string) []string {",
	})
	assertSourceExcludesAll(t, supportSource, []string{
		"func studioCategoryPath(sds *SDSSyncOptions) []string {",
		"func studioAttributes(sds *SDSSyncOptions, trace canonical.FieldTrace) map[string]canonical.Attribute {",
		"func studioSpecifications(sds *SDSSyncOptions) *canonical.ProductSpecs {",
		"func studioVariants(sds *SDSSyncOptions, images []canonical.Image, trace canonical.FieldTrace) []canonical.Variant {",
		"func studioSellingPoints(sds *SDSSyncOptions) []string {",
	})

	for _, path := range listingKitProductionGoFiles(t) {
		if hasFunctionDeclaration(parseListingKitGoFile(t, path), "applyStudioStyleDimension") {
			t.Fatalf("%s should not declare retired applyStudioStyleDimension", path)
		}
	}

	adapterFile := parseListingKitGoFile(t, "sds_canonical_metadata.go")
	if !hasImportPath(adapterFile, "task-processor/internal/product/sourcing/sdspod") {
		t.Fatal("sds_canonical_metadata.go should import sdspod")
	}
	if !hasSelectorCall(adapterFile, "sdspod", "ApplyCanonical") {
		t.Fatal("sds_canonical_metadata.go should call sdspod.ApplyCanonical")
	}
}

package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinSubmitSKUNormalizationSupportFilesOwnHelperFamilies(t *testing.T) {
	t.Parallel()

	homeSrc, err := os.ReadFile("shein_submit_sku_normalization.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_sku_normalization.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"func normalizeSheinStudioSubmitSupplierSKUs(task *Task, pkg *sheinpub.Package, submitRequestID string) bool {",
		"type sheinStudioSupplierSKURename = sheinpub.SupplierSKURename",
		"func resolveStudioSubmitStyleSuffix(task *Task) string {",
		"func sheinStudioStyleID(options *SheinStudioOptions) string {",
		"func adaptSubmitVariantContext(sds *SDSSyncOptions) *sheinpub.SubmitVariantContext {",
		"func adaptSubmitVariantOption(item *SDSSyncVariantOption) *sheinpub.SubmitVariantOption {",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("shein_submit_sku_normalization.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func matchStudioSubmitVariantOption(sds *SDSSyncOptions, draftSKC *SheinSKCRequestDraft, draftSKU *sheinpub.SKUDraft, globalIndex int) (*SDSSyncVariantOption, int) {",
		"func applySheinStudioSupplierSKURenames(pkg *sheinpub.Package, renames []sheinStudioSupplierSKURename) {",
		"func studioSubmitRequestDiscriminator(requestID string) string {",
		"seen := map[string]int{}",
		"for skcIndex := range pkg.DraftPayload.SKCList",
		"pkg.PreviewPayload.SKCList[skcIndex].SKUS[skuIndex].SupplierSKU = newSKU",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("shein_submit_sku_normalization.go should delegate helper family %q", needle)
		}
	}

	assertFileAbsent(t, "shein_submit_sku_variant_support.go")
	for _, needle := range []string{
		"func matchStudioSubmitVariantOption(",
		"func resolveStudioSubmitBaseSKU(",
		"func resolveStudioSubmitVariantDiscriminator(",
		"func inferStudioSubmitBaseSKUFromOld(",
		"func studioSubmitRequiresVariantDiscriminator(",
		"func studioSubmitVariantMatches(",
		"func sheinDraftSKCSaleAttributeValue(",
		"func itoa(",
		"sourceSKU := strings.TrimSpace",
		"colorMatches := make([]int",
		"strings.EqualFold(strings.TrimSpace(item.Color)",
		"styleSuffix := normalizeStyleIDSuffix(styleID)",
		"studioVariantBaseSKUCounts",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("shein_submit_sku_normalization.go should delegate variant detail %q", needle)
		}
	}

	publishingVariantSrc, err := os.ReadFile("../publishing/shein/submit_sku_variant.go")
	if err != nil {
		t.Fatalf("ReadFile(../publishing/shein/submit_sku_variant.go) error = %v", err)
	}
	publishingVariantContent := string(publishingVariantSrc)
	for _, needle := range []string{
		"func MatchSubmitVariantOptionIndex(input *SubmitVariantContext, draftSKCValue string, draftSKU *SKUDraft, globalIndex int) int {",
		"func SubmitVariantMatches(item *SubmitVariantOption, color, size string) bool {",
		"func ResolveSubmitBaseSKU(input *SubmitVariantContext, draftSKU *SKUDraft, match *SubmitVariantOption, oldSKU string) string {",
		"func ResolveSubmitVariantDiscriminator(input *SubmitVariantContext, draftSKU *SKUDraft, match *SubmitVariantOption, matchedIndex, globalIndex int, taskDiscriminator string) string {",
		"func SubmitRequiresVariantDiscriminator(input *SubmitVariantContext, baseSKU string) bool {",
		"func InferSubmitBaseSKUFromOld(oldSKU, styleID string) string {",
	} {
		if !strings.Contains(publishingVariantContent, needle) {
			t.Fatalf("publishing submit_sku_variant.go should contain %q", needle)
		}
	}

	assertFileAbsent(t, "shein_submit_sku_pricing_support.go")

	publishingPricingSrc, err := os.ReadFile("../publishing/shein/submit_sku_pricing.go")
	if err != nil {
		t.Fatalf("ReadFile(../publishing/shein/submit_sku_pricing.go) error = %v", err)
	}
	publishingPricingContent := string(publishingPricingSrc)
	for _, needle := range []string{
		"func ApplyStudioSupplierSKURenames(pkg *Package, renames []SupplierSKURename) {",
		"func ReconcileStudioPricingReferences(pkg *Package) bool {",
		"func StudioPricingSKUAlias(value string) string {",
	} {
		if !strings.Contains(publishingPricingContent, needle) {
			t.Fatalf("publishing submit_sku_pricing.go should contain %q", needle)
		}
	}

	assertFileAbsent(t, "shein_submit_sku_style_support.go")
	for _, needle := range []string{
		"stopwords := map[string]bool",
		"func combineStudioSubmitDiscriminators(values ...string) string {",
		"func studioSubmitRequestDiscriminator(requestID string) string {",
		"tokenizeStudioStyleSuffixWords(value)",
		"b.WriteString(\"R\")",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("shein_submit_sku_normalization.go should delegate style detail %q", needle)
		}
	}
}

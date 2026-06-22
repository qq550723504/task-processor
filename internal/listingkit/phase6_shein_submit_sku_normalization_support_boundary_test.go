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

	variantSrc, err := os.ReadFile("shein_submit_sku_variant_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_sku_variant_support.go) error = %v", err)
	}
	variantContent := string(variantSrc)

	for _, needle := range []string{
		"func matchStudioSubmitVariantOption(sds *SDSSyncOptions, draftSKC *SheinSKCRequestDraft, draftSKU *sheinpub.SKUDraft, globalIndex int) (*SDSSyncVariantOption, int) {",
		"func resolveStudioSubmitBaseSKU(sds *SDSSyncOptions, draftSKU *sheinpub.SKUDraft, match *SDSSyncVariantOption, oldSKU string) string {",
		"func resolveStudioSubmitVariantDiscriminator(sds *SDSSyncOptions, draftSKU *sheinpub.SKUDraft, match *SDSSyncVariantOption, matchedIndex, globalIndex int, taskDiscriminator string) string {",
		"func inferStudioSubmitBaseSKUFromOld(oldSKU, styleID string) string {",
	} {
		if !strings.Contains(variantContent, needle) {
			t.Fatalf("shein_submit_sku_variant_support.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"sourceSKU := strings.TrimSpace",
		"colorMatches := make([]int",
		"strings.EqualFold(strings.TrimSpace(item.Color)",
		"styleSuffix := normalizeStyleIDSuffix(styleID)",
		"studioVariantBaseSKUCounts",
	} {
		if strings.Contains(variantContent, needle) {
			t.Fatalf("shein_submit_sku_variant_support.go should delegate variant detail %q", needle)
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

	styleSrc, err := os.ReadFile("shein_submit_sku_style_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_sku_style_support.go) error = %v", err)
	}
	styleContent := string(styleSrc)

	for _, needle := range []string{
		"func resolveStudioSubmitStyleSuffix(task *Task) string {",
		"func sheinStudioStyleID(options *SheinStudioOptions) string {",
	} {
		if !strings.Contains(styleContent, needle) {
			t.Fatalf("shein_submit_sku_style_support.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"stopwords := map[string]bool",
		"func combineStudioSubmitDiscriminators(values ...string) string {",
		"func studioSubmitRequestDiscriminator(requestID string) string {",
		"tokenizeStudioStyleSuffixWords(value)",
		"b.WriteString(\"R\")",
	} {
		if strings.Contains(styleContent, needle) {
			t.Fatalf("shein_submit_sku_style_support.go should delegate style detail %q", needle)
		}
	}
}

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

	pricingSrc, err := os.ReadFile("shein_submit_sku_pricing_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_sku_pricing_support.go) error = %v", err)
	}
	pricingContent := string(pricingSrc)

	for _, needle := range []string{
		"func applySheinStudioSupplierSKURenames(pkg *sheinpub.Package, renames []sheinStudioSupplierSKURename) {",
		"func reconcileSheinStudioPricingReferences(pkg *sheinpub.Package) bool {",
		"func sheinStudioPricingSKUAlias(value string) string {",
	} {
		if !strings.Contains(pricingContent, needle) {
			t.Fatalf("shein_submit_sku_pricing_support.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"renameMap := make(map[string][]string",
		"func remapSheinPriceOverrides(",
		"func collectSheinRequestDraftSupplierSKUs(",
		"func reconcileSheinPriceOverrideAliases(",
		"func trimSheinStudioPricingStyleLikeSuffix(",
	} {
		if strings.Contains(pricingContent, needle) {
			t.Fatalf("shein_submit_sku_pricing_support.go should delegate pricing detail %q", needle)
		}
	}

	styleSrc, err := os.ReadFile("shein_submit_sku_style_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_sku_style_support.go) error = %v", err)
	}
	styleContent := string(styleSrc)

	for _, needle := range []string{
		"func resolveStudioSubmitStyleSuffix(task *Task) string {",
		"func combineStudioSubmitDiscriminators(values ...string) string {",
		"func studioSubmitRequestDiscriminator(requestID string) string {",
	} {
		if !strings.Contains(styleContent, needle) {
			t.Fatalf("shein_submit_sku_style_support.go should contain %q", needle)
		}
	}
}

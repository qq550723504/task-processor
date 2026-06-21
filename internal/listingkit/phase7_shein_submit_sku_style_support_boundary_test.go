package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinSubmitSKUStyleSupportBoundary(t *testing.T) {
	t.Parallel()

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
		"func remapSheinPriceOverrides(input map[string]float64, renameMap map[string][]string) map[string]float64 {",
		"func collectSheinRequestDraftSupplierSKUs(draft *sheinpub.RequestDraft) []string {",
		"func reconcileSheinPriceOverrideAliases(",
		"func trimSheinStudioPricingStyleLikeSuffix(value string) (string, bool) {",
	} {
		if strings.Contains(pricingContent, needle) {
			t.Fatalf("shein_submit_sku_pricing_support.go should delegate pricing detail %q", needle)
		}
	}

	for _, needle := range []string{
		"func looksLikeStudioSubmitRequestToken(token string) bool {",
		"func looksLikeStudioSubmitTaskToken(token string) bool {",
		"func resolveStudioSubmitStyleSuffix(task *Task) string {",
		"func sheinStudioStyleID(options *SheinStudioOptions) string {",
		"func deriveStudioSubmitStyleSuffix(values ...string) string {",
		"func tokenizeStudioStyleSuffixWords(value string) []string {",
		"func studioSubmitTaskDiscriminator(taskID string) string {",
		"func studioSubmitRequestDiscriminator(requestID string) string {",
		"func combineStudioSubmitDiscriminators(values ...string) string {",
	} {
		if strings.Contains(pricingContent, needle) {
			t.Fatalf("shein_submit_sku_pricing_support.go should delegate style support helper %q", needle)
		}
	}

	styleSrc, err := os.ReadFile("shein_submit_sku_style_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_sku_style_support.go) error = %v", err)
	}
	styleContent := string(styleSrc)

	for _, needle := range []string{
		"func looksLikeStudioSubmitRequestToken(token string) bool {",
		"func looksLikeStudioSubmitTaskToken(token string) bool {",
		"func resolveStudioSubmitStyleSuffix(task *Task) string {",
		"func deriveStudioSubmitStyleSuffix(values ...string) string {",
		"func tokenizeStudioStyleSuffixWords(value string) []string {",
		"func studioSubmitTaskDiscriminator(taskID string) string {",
		"func studioSubmitRequestDiscriminator(requestID string) string {",
		"func combineStudioSubmitDiscriminators(values ...string) string {",
	} {
		if !strings.Contains(styleContent, needle) {
			t.Fatalf("shein_submit_sku_style_support.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"stopwords := map[string]bool",
		"token = strings.TrimSpace(strings.ToUpper(token))",
		"var builder strings.Builder",
		"b.WriteString(\"T\")",
		"b.WriteString(\"R\")",
		"strings.Join(parts, \"-\")",
	} {
		if strings.Contains(styleContent, needle) {
			t.Fatalf("shein_submit_sku_style_support.go should delegate style detail %q", needle)
		}
	}
}

package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinSubmitSKUStyleSupportBoundary(t *testing.T) {
	t.Parallel()

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
		if strings.Contains(publishingPricingContent, needle) {
			t.Fatalf("shein_submit_sku_pricing_support.go should delegate style support helper %q", needle)
		}
	}

	assertFileAbsent(t, "shein_submit_sku_style_support.go")

	normalizationSrc, err := os.ReadFile("shein_submit_sku_normalization.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_sku_normalization.go) error = %v", err)
	}
	normalizationContent := string(normalizationSrc)

	for _, needle := range []string{
		"func resolveStudioSubmitStyleSuffix(task *Task) string {",
		"func sheinStudioStyleID(options *SheinStudioOptions) string {",
	} {
		if !strings.Contains(normalizationContent, needle) {
			t.Fatalf("shein_submit_sku_normalization.go should contain app adapter %q", needle)
		}
	}
	for _, needle := range []string{
		"func looksLikeStudioSubmitRequestToken(token string) bool {",
		"func looksLikeStudioSubmitTaskToken(token string) bool {",
		"func deriveStudioSubmitStyleSuffix(values ...string) string {",
		"func tokenizeStudioStyleSuffixWords(value string) []string {",
		"func studioSubmitTaskDiscriminator(taskID string) string {",
		"func studioSubmitRequestDiscriminator(requestID string) string {",
		"func combineStudioSubmitDiscriminators(values ...string) string {",
	} {
		if strings.Contains(normalizationContent, needle) {
			t.Fatalf("shein_submit_sku_normalization.go should delegate style wrapper %q", needle)
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
		if strings.Contains(normalizationContent, needle) {
			t.Fatalf("shein_submit_sku_normalization.go should delegate style detail %q", needle)
		}
	}

	publishingStyleSrc, err := os.ReadFile("../publishing/shein/submit_sku_style.go")
	if err != nil {
		t.Fatalf("ReadFile(../publishing/shein/submit_sku_style.go) error = %v", err)
	}
	publishingStyleContent := string(publishingStyleSrc)
	for _, needle := range []string{
		"func LooksLikeSubmitRequestToken(token string) bool {",
		"func LooksLikeSubmitTaskToken(token string) bool {",
		"func DeriveSubmitStyleSuffix(values ...string) string {",
		"func TokenizeStyleSuffixWords(value string) []string {",
		"func SubmitTaskDiscriminator(taskID string) string {",
		"func SubmitRequestDiscriminator(requestID string) string {",
		"func CombineSubmitDiscriminators(values ...string) string {",
	} {
		if !strings.Contains(publishingStyleContent, needle) {
			t.Fatalf("publishing submit_sku_style.go should contain %q", needle)
		}
	}
}

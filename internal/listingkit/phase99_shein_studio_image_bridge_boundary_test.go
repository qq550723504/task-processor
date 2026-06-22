package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinStudioImageBridgeCallsPublishingDirectly(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "shein_studio_ai_product_images.go")

	studioSrc, err := os.ReadFile("shein_studio_images.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_studio_images.go) error = %v", err)
	}
	studioContent := string(studioSrc)
	for _, needle := range []string{
		"sheinpub.AppendAIProductImages(pkg, productImages, sourceImages)",
		"sheinpub.ReplaceImagesWithAIProductImages(pkg, productImages, sourceImages)",
	} {
		if !strings.Contains(studioContent, needle) {
			t.Fatalf("shein_studio_images.go should contain %q", needle)
		}
	}
	for _, forbidden := range []string{
		"appendAIProductImagesToShein(",
		"replaceSheinImagesWithAIProductImages(",
	} {
		if strings.Contains(studioContent, forbidden) {
			t.Fatalf("shein_studio_images.go should not call wrapper %q", forbidden)
		}
	}

	variantSrc, err := os.ReadFile("shein_studio_variant_images.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_studio_variant_images.go) error = %v", err)
	}
	variantContent := string(variantSrc)
	for _, needle := range []string{
		"sheinpub.NormalizeVariantImageKey(",
		"sheinpub.FindVariantImageSetForRequestSKC(",
		"sheinpub.FindVariantImageSetForPackageSKC(",
		"sheinpub.ImageSetFromAIProductImages(",
	} {
		if !strings.Contains(variantContent, needle) {
			t.Fatalf("shein_studio_variant_images.go should contain %q", needle)
		}
	}
	for _, forbidden := range []string{
		"func findVariantImageSetForSKC(",
		"func findVariantImageSetForSKCPackage(",
		"func normalizeVariantImageKey(",
		"imageSetFromAIProductImages(",
	} {
		if strings.Contains(variantContent, forbidden) {
			t.Fatalf("shein_studio_variant_images.go should not keep wrapper %q", forbidden)
		}
	}
}

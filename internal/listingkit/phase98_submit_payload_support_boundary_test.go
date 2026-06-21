package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinSubmitPayloadSupportFilesOwnHelperFamilies(t *testing.T) {
	t.Parallel()

	homeSrc, err := os.ReadFile("shein_submit_payload.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_payload.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"func prepareSheinProductForNewSubmit(product *sheinproduct.Product) {",
		"func prepareSheinProductForSubmit(product *sheinproduct.Product, settings SheinSettings) {",
		"func normalizeSheinSubmitCollections(product *sheinproduct.Product) {",
		"func normalizeSheinSubmitExtra(product *sheinproduct.Product) {",
		"func finalizeSheinSubmitTransportFields(product *sheinproduct.Product) {",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("shein_submit_payload.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func ensureSheinSubmitSites(product *sheinproduct.Product, settings SheinSettings) {",
		"func normalizeSheinSubmitImages(product *sheinproduct.Product) {",
		"func deriveSheinSubmitProductSupplierCode(product *sheinproduct.Product) string {",
		"func validateSheinProductPublishPayload(product *sheinproduct.Product) error {",
		"product.SPUName = \"\"",
		"product.SourceSystem = \"listingkit\"",
		"product.SupplierCode = deriveSheinSubmitProductSupplierCode(product)",
		"normalizeSheinSubmitCollections(product)",
		"ensureSheinSubmitSKUs(product, settings)",
		"product.BrandSeriesList = []string{}",
		"product.Extra.SPUTag = []string{}",
		"product.Extra.ControlPriceData = map[string]string{}",
		"sku.CompetingCostPriceImages = []any{}",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("shein_submit_payload.go should delegate helper seam %q", needle)
		}
	}
	for _, needle := range []string{
		"sheinpub.PrepareProductForNewSubmit(product)",
		"sheinpub.PrepareProductForSubmit(product, sheinSubmitPayloadSettings(settings))",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("shein_submit_payload.go should delegate submit product preparation via %q", needle)
		}
	}

	assertFileAbsent(t, "shein_submit_payload_site_support.go")

	publishingSiteSrc, err := os.ReadFile("../publishing/shein/submit_site_sku_policy.go")
	if err != nil {
		t.Fatalf("ReadFile(../publishing/shein/submit_site_sku_policy.go) error = %v", err)
	}
	publishingSiteContent := string(publishingSiteSrc)
	for _, needle := range []string{
		"func EnsureSubmitSites(product *sheinproduct.Product, settings SubmitPayloadSettings) {",
		"func EnsureSubmitSKUs(product *sheinproduct.Product, settings SubmitPayloadSettings) {",
		"func NormalizeSubmitWeight(sku *sheinproduct.SKU) {",
		"func SubmitPreferredWarehouseCode(settings SubmitPayloadSettings) string {",
	} {
		if !strings.Contains(publishingSiteContent, needle) {
			t.Fatalf("publishing submit_site_sku_policy.go should contain %q", needle)
		}
	}

	assertFileAbsent(t, "shein_submit_payload_image_support.go")

	publishingImageSrc, err := os.ReadFile("../publishing/shein/submit_payload_images.go")
	if err != nil {
		t.Fatalf("ReadFile(../publishing/shein/submit_payload_images.go) error = %v", err)
	}
	publishingImageContent := string(publishingImageSrc)
	for _, needle := range []string{
		"func NormalizeSubmitImages(product *sheinproduct.Product) {",
		"func NormalizeSubmitSKUImages(skc *sheinproduct.SKC) {",
		"func NormalizeSubmitGalleryImages(images []sheinproduct.ImageDetail, includeColorBlock bool) []sheinproduct.ImageDetail {",
		"func DedupeImagesByURL(images []sheinproduct.ImageDetail) []sheinproduct.ImageDetail {",
	} {
		if !strings.Contains(publishingImageContent, needle) {
			t.Fatalf("publishing submit_payload_images.go should contain %q", needle)
		}
	}

	supplierSrc, err := os.ReadFile("shein_submit_payload_supplier_validation_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_payload_supplier_validation_support.go) error = %v", err)
	}
	supplierContent := string(supplierSrc)

	for _, needle := range []string{
		"func deriveSheinSubmitProductSupplierCode(product *sheinproduct.Product) string {",
		"func validateSheinProductPublishPayload(product *sheinproduct.Product) error {",
	} {
		if !strings.Contains(supplierContent, needle) {
			t.Fatalf("shein_submit_payload_supplier_validation_support.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"func deriveSheinSubmitSupplierCodeFromSKU(supplierSKU string) string {",
		"func looksLikeRawBaseSupplierCode(value string) bool {",
		"func normalizeSheinSubmitStyleSuffix(value string) string {",
		"case 5:",
		"case 6:",
	} {
		if strings.Contains(supplierContent, needle) {
			t.Fatalf("shein_submit_payload_supplier_validation_support.go should delegate publishing policy detail %q", needle)
		}
	}
}

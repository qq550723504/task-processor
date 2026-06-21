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
		"product.BrandSeriesList = []string{}",
		"product.Extra.SPUTag = []string{}",
		"product.Extra.ControlPriceData = map[string]string{}",
		"sku.CompetingCostPriceImages = []any{}",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("shein_submit_payload.go should delegate helper seam %q", needle)
		}
	}

	siteSrc, err := os.ReadFile("shein_submit_payload_site_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_payload_site_support.go) error = %v", err)
	}
	siteContent := string(siteSrc)

	for _, needle := range []string{
		"func ensureSheinSubmitSites(product *sheinproduct.Product, settings SheinSettings) {",
		"func ensureSheinSubmitSKUs(product *sheinproduct.Product, settings SheinSettings) {",
		"func normalizeSheinSubmitWeight(sku *sheinproduct.SKU) {",
	} {
		if !strings.Contains(siteContent, needle) {
			t.Fatalf("shein_submit_payload_site_support.go should contain %q", needle)
		}
	}
	for _, needle := range []string{
		"defaultSheinSKCShelfWay",
		"convertSheinWeightToGrams",
		"roundSheinWeightGrams",
		"sku.StockInfoList = []sheinproduct.StockInfo",
		"case \"kg\", \"kilogram\", \"kilograms\":",
	} {
		if strings.Contains(siteContent, needle) {
			t.Fatalf("shein_submit_payload_site_support.go should delegate site/SKU policy detail %q", needle)
		}
	}

	imageSrc, err := os.ReadFile("shein_submit_payload_image_support.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_submit_payload_image_support.go) error = %v", err)
	}
	imageContent := string(imageSrc)

	for _, needle := range []string{
		"func normalizeSheinSubmitImages(product *sheinproduct.Product) {",
		"func normalizeSheinSubmitSKUImages(skc *sheinproduct.SKC) {",
		"func normalizeSheinSubmitGalleryImages(images []sheinproduct.ImageDetail, includeColorBlock bool) []sheinproduct.ImageDetail {",
		"func dedupeSheinImagesByURL(images []sheinproduct.ImageDetail) []sheinproduct.ImageDetail {",
	} {
		if !strings.Contains(imageContent, needle) {
			t.Fatalf("shein_submit_payload_image_support.go should contain %q", needle)
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

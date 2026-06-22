package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestSheinSubmitPayloadSupportFilesOwnHelperFamilies(t *testing.T) {
	t.Parallel()

	assertFileAbsent(t, "shein_submit_payload.go")

	settingsSrc, err := os.ReadFile("shein_settings.go")
	if err != nil {
		t.Fatalf("ReadFile(shein_settings.go) error = %v", err)
	}
	settingsContent := string(settingsSrc)
	for _, needle := range []string{
		"func sheinSubmitPayloadSettings(settings SheinSettings) sheinpub.SubmitPayloadSettings {",
		"Site:          settings.Site",
		"WarehouseCode: settings.WarehouseCode",
	} {
		if !strings.Contains(settingsContent, needle) {
			t.Fatalf("shein_settings.go should contain %q", needle)
		}
	}

	homeSrc, err := os.ReadFile("task_submission_execution_product.go")
	if err != nil {
		t.Fatalf("ReadFile(task_submission_execution_product.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"func ensureSheinSubmitSites(product *sheinproduct.Product, settings SheinSettings) {",
		"func normalizeSheinSubmitImages(product *sheinproduct.Product) {",
		"func deriveSheinSubmitProductSupplierCode(product *sheinproduct.Product) string {",
		"func validateSheinProductPublishPayload(product *sheinproduct.Product) error {",
		"product.SPUName = \"\"",
		"product.SourceSystem = \"listingkit\"",
		"product.SupplierCode = deriveSheinSubmitProductSupplierCode(product)",
		"normalizeSheinSubmitCollections(product)",
		"func normalizeSheinSubmitCollections(product *sheinproduct.Product) {",
		"func normalizeSheinSubmitExtra(product *sheinproduct.Product) {",
		"func finalizeSheinSubmitTransportFields(product *sheinproduct.Product) {",
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
		"sheinpub.PrepareProductForSubmit(submitProduct, sheinSubmitPayloadSettings(s.resolveSubmitSettings(runtimeCtx, task)))",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("task_submission_execution_product.go should delegate submit product preparation via %q", needle)
		}
	}
	publishingNormalizeSrc, err := os.ReadFile("../publishing/shein/submit_payload_normalize.go")
	if err != nil {
		t.Fatalf("ReadFile(../publishing/shein/submit_payload_normalize.go) error = %v", err)
	}
	publishingNormalizeContent := string(publishingNormalizeSrc)
	for _, needle := range []string{
		"func NormalizeSubmitCollections(product *sheinproduct.Product) {",
		"func NormalizeSubmitExtra(product *sheinproduct.Product) {",
		"func FinalizeSubmitTransportFields(product *sheinproduct.Product) {",
	} {
		if !strings.Contains(publishingNormalizeContent, needle) {
			t.Fatalf("publishing submit_payload_normalize.go should contain %q", needle)
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

	assertFileAbsent(t, "shein_submit_payload_supplier_validation_support.go")

	publishingPolicySrc, err := os.ReadFile("../publishing/shein/submit_payload_policy.go")
	if err != nil {
		t.Fatalf("ReadFile(../publishing/shein/submit_payload_policy.go) error = %v", err)
	}
	publishingPolicyContent := string(publishingPolicySrc)
	for _, needle := range []string{
		"func DeriveSubmitProductSupplierCode(product *sheinproduct.Product) string {",
		"func ValidateProductPublishPayload(product *sheinproduct.Product) error {",
	} {
		if !strings.Contains(publishingPolicyContent, needle) {
			t.Fatalf("publishing submit_payload_policy.go should contain %q", needle)
		}
	}
}

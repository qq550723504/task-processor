package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestRevisionApplySheinSupportFilesOwnHelperFamilies(t *testing.T) {
	t.Parallel()

	homeSrc, err := os.ReadFile("revision_apply_shein.go")
	if err != nil {
		t.Fatalf("ReadFile(revision_apply_shein.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"func applySheinRevision(pkg *sheinpub.Package, req *SheinRevisionInput) {",
		"func normalizeSheinSaleAttributeState(pkg *sheinpub.Package) {",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("revision_apply_shein.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func applySheinCategoryResolutionPatch(pkg *sheinpub.Package, patch *SheinCategoryResolutionPatch) {",
		"func applySheinSaleAttributeResolutionPatch(pkg *sheinpub.Package, patch *SheinSaleAttributeResolutionPatch) {",
		"func applySheinSKCRevisionPatches(pkg *sheinpub.Package, patches []SheinSKCRevisionPatch) {",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("revision_apply_shein.go should delegate helper family %q", needle)
		}
	}

	resolutionSrc, err := os.ReadFile("revision_apply_shein_resolution_support.go")
	if err != nil {
		t.Fatalf("ReadFile(revision_apply_shein_resolution_support.go) error = %v", err)
	}
	resolutionContent := string(resolutionSrc)

	for _, needle := range []string{
		"func applySheinCategoryResolutionPatch(pkg *sheinpub.Package, patch *SheinCategoryResolutionPatch) {",
		"func applySheinAttributeResolutionPatch(pkg *sheinpub.Package, patch *SheinAttributeResolutionPatch) {",
		"func applySheinSaleAttributeResolutionPatch(pkg *sheinpub.Package, patch *SheinSaleAttributeResolutionPatch) {",
		"func cloneSheinResolvedSaleAttributeMap(src map[string]sheinpub.ResolvedSaleAttribute) map[string]sheinpub.ResolvedSaleAttribute {",
	} {
		if !strings.Contains(resolutionContent, needle) {
			t.Fatalf("revision_apply_shein_resolution_support.go should contain %q", needle)
		}
	}

	skuSrc, err := os.ReadFile("revision_apply_shein_sku_support.go")
	if err != nil {
		t.Fatalf("ReadFile(revision_apply_shein_sku_support.go) error = %v", err)
	}
	skuContent := string(skuSrc)

	for _, needle := range []string{
		"func applySheinSKCRevisionPatches(pkg *sheinpub.Package, patches []SheinSKCRevisionPatch) {",
		"func applySheinSKURevisionPatches(pkg *sheinpub.Package, draft *sheinpub.SKCRequestDraft, pkgSKC *sheinpub.SKCPackage, patches []SheinSKURevisionPatch) {",
	} {
		if !strings.Contains(skuContent, needle) {
			t.Fatalf("revision_apply_shein_sku_support.go should contain %q", needle)
		}
	}
}

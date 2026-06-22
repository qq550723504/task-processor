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

	for _, needle := range []string{
		"sheinworkspace.ApplyCategoryResolutionPatch(pkg, req.CategoryResolution)",
		"sheinworkspace.ApplyAttributeResolutionPatch(pkg, req.AttributeResolution)",
		"sheinworkspace.ApplySaleAttributeResolutionPatch(pkg, req.SaleAttributeResolution)",
		"sheinworkspace.ApplySKCRevisionPatches(pkg, req.SKCPatches)",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("revision_apply_shein.go should call workspace directly via %q", needle)
		}
	}

	for _, needle := range []string{
		"func applySheinCategoryResolutionPatch(pkg *sheinpub.Package, patch *SheinCategoryResolutionPatch) {",
		"func applySheinAttributeResolutionPatch(pkg *sheinpub.Package, patch *SheinAttributeResolutionPatch) {",
		"func applySheinSaleAttributeResolutionPatch(pkg *sheinpub.Package, patch *SheinSaleAttributeResolutionPatch) {",
		"sheinworkspace.ApplyCategoryResolutionPatch(pkg, patch)",
		"sheinworkspace.ApplyAttributeResolutionPatch(pkg, patch)",
		"sheinworkspace.ApplySaleAttributeResolutionPatch(pkg, patch)",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("revision_apply_shein.go should not keep resolution wrapper detail %q", needle)
		}
	}
	assertFileAbsent(t, "revision_apply_shein_resolution_support.go")

	for _, needle := range []string{
		"func applySheinSKCRevisionPatches(pkg *sheinpub.Package, patches []SheinSKCRevisionPatch) {",
		"sheinworkspace.ApplySKCRevisionPatches(pkg, patches)",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("revision_apply_shein.go should not keep SKC wrapper detail %q", needle)
		}
	}
	for _, needle := range []string{
		"func applySheinSKURevisionPatches(pkg *sheinpub.Package, draft *sheinpub.SKCRequestDraft, pkgSKC *sheinpub.SKCPackage, patches []SheinSKURevisionPatch) {",
		"sheinworkspace.ApplySKURevisionPatches(pkg, draft, pkgSKC, patches)",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("revision_apply_shein.go should not keep unused SKU wrapper %q", needle)
		}
	}
	assertFileAbsent(t, "revision_apply_shein_sku_support.go")

	workspaceSrc, err := os.ReadFile("../marketplace/shein/workspace/revision_apply_patch.go")
	if err != nil {
		t.Fatalf("ReadFile(../marketplace/shein/workspace/revision_apply_patch.go) error = %v", err)
	}
	workspaceContent := string(workspaceSrc)

	for _, needle := range []string{
		"func ApplyCategoryResolutionPatch(pkg *sheinpub.Package, patch *CategoryResolutionPatch) {",
		"func ApplyAttributeResolutionPatch(pkg *sheinpub.Package, patch *AttributeResolutionPatch) {",
		"func ApplySaleAttributeResolutionPatch(pkg *sheinpub.Package, patch *SaleAttributeResolutionPatch) {",
		"func ApplySKCRevisionPatches(pkg *sheinpub.Package, patches []SKCRevisionPatch) {",
		"func ApplySKURevisionPatches(pkg *sheinpub.Package, draft *sheinpub.SKCRequestDraft, pkgSKC *sheinpub.SKCPackage, patches []SKURevisionPatch) {",
	} {
		if !strings.Contains(workspaceContent, needle) {
			t.Fatalf("workspace revision apply patch should contain %q", needle)
		}
	}
}

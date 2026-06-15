package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestRevisionManualSaleAttributeSupportFilesOwnHelperFamilies(t *testing.T) {
	t.Parallel()

	homeSrc, err := os.ReadFile("service_revision_manual_sale_attributes.go")
	if err != nil {
		t.Fatalf("ReadFile(service_revision_manual_sale_attributes.go) error = %v", err)
	}
	homeContent := string(homeSrc)

	for _, needle := range []string{
		"func (s *service) resolveManualSheinSaleAttributeValueIDs(",
		"func resolveManualSheinSaleAttributeValueIDs(",
		"func resolveManualSheinSKUAttributeValueWithVariants(",
	} {
		if !strings.Contains(homeContent, needle) {
			t.Fatalf("service_revision_manual_sale_attributes.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"func backfillManualSheinSaleAttributeAssignments(pkg *SheinPackage, req *SheinRevisionInput) {",
		"func manualSheinComparableSourceValues(sourceValue string) []string {",
		"func lookupSheinSKUSourceValue(pkg *SheinPackage, supplierCode, supplierSKU, dimension string) string {",
	} {
		if strings.Contains(homeContent, needle) {
			t.Fatalf("service_revision_manual_sale_attributes.go should delegate helper family %q", needle)
		}
	}

	assignSrc, err := os.ReadFile("service_revision_manual_sale_attributes_assignment_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_revision_manual_sale_attributes_assignment_support.go) error = %v", err)
	}
	assignContent := string(assignSrc)

	for _, needle := range []string{
		"func backfillManualSheinSaleAttributeAssignments(pkg *SheinPackage, req *SheinRevisionInput) {",
		"func syncSheinManualSaleAttributeResolution(req *SheinRevisionInput) {",
		"func manualSheinSaleAttributesNeedRemoteResolution(req *SheinRevisionInput) bool {",
		"func flattenSheinAttributeTemplatesByID(info *sheinattribute.AttributeTemplateInfo) map[int]sheinattribute.AttributeInfo {",
	} {
		if !strings.Contains(assignContent, needle) {
			t.Fatalf("service_revision_manual_sale_attributes_assignment_support.go should contain %q", needle)
		}
	}

	sourceSrc, err := os.ReadFile("service_revision_manual_sale_attributes_source_support.go")
	if err != nil {
		t.Fatalf("ReadFile(service_revision_manual_sale_attributes_source_support.go) error = %v", err)
	}
	sourceContent := string(sourceSrc)

	for _, needle := range []string{
		"func manualSheinComparableSourceValues(sourceValue string) []string {",
		"func lookupSheinSKCSourceValue(pkg *SheinPackage, supplierCode, dimension string) string {",
		"func lookupSheinSKUSourceValue(pkg *SheinPackage, supplierCode, supplierSKU, dimension string) string {",
		"func dedupeCustomAttributeRelations(relations []sheinattribute.CustomAttributeRelation) []sheinattribute.CustomAttributeRelation {",
	} {
		if !strings.Contains(sourceContent, needle) {
			t.Fatalf("service_revision_manual_sale_attributes_source_support.go should contain %q", needle)
		}
	}
}

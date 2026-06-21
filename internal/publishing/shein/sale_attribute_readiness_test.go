package shein

import (
	"testing"

	"task-processor/internal/publishing/common"
)

func TestSecondarySaleAttributeRequiredRequiresMultiSKUSecondarySourceAndTemplate(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		SaleAttributeResolution: &SaleAttributeResolution{
			PrimaryAttributeID:       1001,
			SecondarySourceDimension: "尺码",
			SourceDimensions: []SourceVariantDimension{{
				Name:          "size",
				DistinctCount: 2,
			}},
			TemplateOptions: []SaleAttributeTemplateOption{{
				AttributeID: 2002,
				Name:        "Size",
			}},
		},
		DraftPayload: &RequestDraft{SKCList: []SKCRequestDraft{{
			SKUList: []SKUDraft{{SupplierSKU: "SKU-1"}, {SupplierSKU: "SKU-2"}},
		}}},
	}

	if !SecondarySaleAttributeRequired(pkg) {
		t.Fatal("SecondarySaleAttributeRequired = false, want true")
	}

	pkg.SaleAttributeResolution.TemplateOptions[0].AttributeID = 1001
	if SecondarySaleAttributeRequired(pkg) {
		t.Fatal("SecondarySaleAttributeRequired = true without secondary template candidate")
	}
}

func TestSaleAttributesReadinessFailureReasonsReportsSKUAttributeBlockers(t *testing.T) {
	t.Parallel()

	valueID := 10
	pkg := &Package{
		SaleAttributeResolution: &SaleAttributeResolution{
			Status:                   "resolved",
			PrimaryAttributeID:       1001,
			SecondaryAttributeID:     2002,
			SecondarySourceDimension: "size",
		},
		DraftPayload: &RequestDraft{SKCList: []SKCRequestDraft{{
			SupplierCode:  "SKC-1",
			SaleAttribute: &ResolvedSaleAttribute{AttributeID: 1001, AttributeValueID: &valueID},
			SKUList: []SKUDraft{{
				SupplierSKU: "SKU-1",
			}},
		}}},
	}

	reasons := SaleAttributesReadinessFailureReasons(pkg)

	if len(reasons) != 1 || reasons[0] != `sku "SKU-1" is missing sale_attributes` {
		t.Fatalf("reasons = %#v, want missing sku sale attributes", reasons)
	}
}

func TestSaleAttributesReadyForSubmitAcceptsResolvedSKCAndSKUAttributes(t *testing.T) {
	t.Parallel()

	skcValueID := 10
	skuValueID := 20
	pkg := &Package{
		SaleAttributeResolution: &SaleAttributeResolution{
			Status:               "resolved",
			PrimaryAttributeID:   1001,
			SecondaryAttributeID: 2002,
		},
		DraftPayload: &RequestDraft{SKCList: []SKCRequestDraft{{
			SupplierCode:  "SKC-1",
			SaleAttribute: &ResolvedSaleAttribute{AttributeID: 1001, AttributeValueID: &skcValueID},
			SKUList: []SKUDraft{{
				SupplierSKU: "SKU-1",
				SaleAttributes: []ResolvedSaleAttribute{{
					AttributeID:      2002,
					AttributeValueID: &skuValueID,
				}},
			}},
		}}},
	}

	if !SaleAttributesReadyForSubmit(pkg) {
		t.Fatalf("SaleAttributesReadyForSubmit = false; reasons=%#v", SaleAttributesReadinessFailureReasons(pkg))
	}
}

func TestHasBlockingPendingAttributesReportsRequiredAndManualPendingAttributes(t *testing.T) {
	t.Parallel()

	if !HasBlockingPendingAttributes(&Package{}) {
		t.Fatal("HasBlockingPendingAttributes without resolution = false, want true")
	}
	if !HasBlockingPendingAttributes(&Package{AttributeResolution: &AttributeResolution{
		PendingAttributeCandidates: []PendingAttributeCandidate{{Required: true}},
	}}) {
		t.Fatal("HasBlockingPendingAttributes required candidate = false, want true")
	}
	if !HasBlockingPendingAttributes(&Package{AttributeResolution: &AttributeResolution{
		PendingAttributes: []common.Attribute{{Name: "Material"}},
	}}) {
		t.Fatal("HasBlockingPendingAttributes manual pending attribute = false, want true")
	}
	if HasBlockingPendingAttributes(&Package{AttributeResolution: &AttributeResolution{}}) {
		t.Fatal("HasBlockingPendingAttributes empty resolution = true, want false")
	}
}

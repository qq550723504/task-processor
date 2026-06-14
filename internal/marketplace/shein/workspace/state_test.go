package workspace

import (
	"testing"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestFilterManualReviewNotesDropsAutoNotes(t *testing.T) {
	notes := append([]string{AutoReviewNotes[0], "需要补充品牌说明", AutoReviewNotes[1]}, "需要补充品牌说明")

	filtered := FilterManualReviewNotes(notes)

	if len(filtered) != 1 || filtered[0] != "需要补充品牌说明" {
		t.Fatalf("filtered = %#v", filtered)
	}
}

func TestBuildEditorDirtyHintsCollectsUniqueFields(t *testing.T) {
	productTypeID := 99
	hints := BuildEditorDirtyHints(&sheinpub.Package{
		SpuName:            "SPU",
		ProductNameEn:      "Name",
		CategoryName:       "Dress",
		CategoryPath:       []string{"Women", "Dresses"},
		CategoryID:         123,
		CategoryIDList:     []int{1, 2, 3},
		ProductTypeID:      &productTypeID,
		TopCategoryID:      456,
		CategoryResolution: &sheinpub.CategoryResolution{Status: "resolved"},
		ProductAttributes:  []common.Attribute{{Name: "Material", Value: "Cotton"}},
		ResolvedAttributes: []sheinpub.ResolvedAttribute{{AttributeID: 1}},
		AttributeResolution: &sheinpub.AttributeResolution{
			Status: "resolved",
		},
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{Status: "resolved"},
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{SupplierCode: "SKC-1"}},
		},
	})

	if hints == nil {
		t.Fatal("expected hints")
	}
	if len(hints.Sections) != 4 {
		t.Fatalf("sections = %d, want 4", len(hints.Sections))
	}
	if len(hints.EditableFields) == 0 || len(hints.DefaultChangedFields) == 0 {
		t.Fatalf("hints = %#v", hints)
	}
}

func TestIsSaleAttributeResolvedRequiresResolvedDraftValues(t *testing.T) {
	valueID := 2001
	pkg := &sheinpub.Package{
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 1001,
		},
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "SKC-1",
				SaleAttribute: &sheinpub.ResolvedSaleAttribute{
					AttributeID:      1001,
					AttributeValueID: &valueID,
				},
			}},
		},
	}

	if !IsSaleAttributeResolved(pkg) {
		t.Fatal("expected resolved sale attributes")
	}

	pkg.RequestDraft.SKCList[0].SaleAttribute.AttributeValueID = nil
	if IsSaleAttributeResolved(pkg) {
		t.Fatal("expected unresolved sale attributes when value id is missing")
	}
}

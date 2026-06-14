package workspace

import (
	"testing"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildEditorProgressCountsResolvedSections(t *testing.T) {
	productTypeID := 101
	valueID := 2001
	progress := BuildEditorProgress(&sheinpub.Package{
		SpuName:       "Bottle",
		BrandName:     "Brand",
		Description:   "Desc",
		Images:        &common.ImageSet{MainImage: "main.jpg"},
		CategoryPath:  []string{"Home", "Kitchen"},
		CategoryID:    12,
		ProductTypeID: &productTypeID,
		CategoryResolution: &sheinpub.CategoryResolution{
			Status: "resolved",
		},
		ProductAttributes: []common.Attribute{
			{Name: "Material", Value: "Glass"},
		},
		AttributeResolution: &sheinpub.AttributeResolution{
			Status:        "resolved",
			ResolvedCount: 1,
		},
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 99,
		},
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "SKC-1",
				SaleAttribute: &sheinpub.ResolvedSaleAttribute{
					AttributeID:      99,
					AttributeValueID: &valueID,
				},
			}},
		},
	}, 12)

	if progress == nil {
		t.Fatal("expected progress")
	}
	if progress.Completed != 11 || progress.Total != 12 {
		t.Fatalf("progress = %+v", progress)
	}
	if progress.Unresolved != 0 {
		t.Fatalf("unresolved = %d, want 0", progress.Unresolved)
	}
}

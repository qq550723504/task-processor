package workspace

import (
	"testing"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildEditorContextBuildsMarketplaceProjection(t *testing.T) {
	productTypeID := 10
	attrValueID := 20
	ctx := BuildEditorContext(&sheinpub.Package{
		SpuName:       "SPU",
		ProductNameEn: "Bottle",
		BrandName:     "Brand",
		Description:   "Desc",
		Images:        &common.ImageSet{MainImage: "main.jpg"},
		ReviewNotes:   []string{"check note"},
		CategoryID:    123,
		ProductTypeID: &productTypeID,
		CategoryPath:  []string{"Home", "Kitchen"},
		CategoryResolution: &sheinpub.CategoryResolution{
			Status: "resolved",
			Source: "suggest_category",
		},
		ResolvedAttributes: []sheinpub.ResolvedAttribute{{
			Name:             "Material",
			Value:            "Glass",
			AttributeID:      1,
			AttributeValueID: &attrValueID,
		}},
		AttributeResolution: &sheinpub.AttributeResolution{
			Status:        "resolved",
			Source:        "template",
			ResolvedCount: 1,
		},
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:             "resolved",
			Source:             "sale_attribute_templates",
			PrimaryAttributeID: 2,
		},
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{SupplierCode: "SKC-1"}},
		},
	})

	if ctx == nil || ctx.Basics == nil || ctx.Category == nil || ctx.Attributes == nil || ctx.SaleAttributes == nil {
		t.Fatalf("context = %#v", ctx)
	}
	if ctx.RevisionSkeleton == nil || ctx.Progress == nil || ctx.DirtyHints == nil {
		t.Fatalf("context = %#v", ctx)
	}
}

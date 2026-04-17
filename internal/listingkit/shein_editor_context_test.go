package listingkit

import "testing"

func TestBuildSheinEditorContextIncludesSuggestedPatches(t *testing.T) {
	t.Parallel()

	productTypeID := 8899
	valueID := 301
	skuValueID := 202
	pkg := &SheinPackage{
		SpuName:       "Bottle",
		ProductNameEn: "Bottle",
		BrandName:     "Demo",
		Description:   "desc",
		Images: &PlatformImageSet{
			MainImage: "https://cdn.example.com/main.jpg",
		},
		CategoryPath:   []string{"Home", "Kitchen", "Bottle"},
		CategoryID:     7788,
		CategoryIDList: []int{100, 200, 7788},
		ProductTypeID:  &productTypeID,
		CategoryResolution: &SheinCategoryResolution{
			Status:      "resolved",
			Source:      "suggest_category",
			MatchedPath: []string{"Home", "Kitchen", "Bottle"},
		},
		ProductAttributes: []PlatformAttribute{
			{Name: "material", Value: "stainless steel"},
		},
		ResolvedAttributes: []SheinResolvedAttribute{{
			Name:             "material",
			Value:            "stainless steel",
			AttributeID:      7001,
			AttributeValueID: &valueID,
		}},
		AttributeResolution: &SheinAttributeResolution{
			Status:        "resolved",
			ResolvedCount: 1,
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			Status:             "resolved",
			Source:             "sale_attribute_templates",
			PrimaryAttributeID: 501,
			SelectionSummary:   []string{"颜色作为 SKC"},
			Candidates: []SheinSaleAttributeCandidateInfo{{
				Name:          "Color",
				AttributeID:   501,
				SelectedScope: "skc",
				PrimaryScore:  12,
				Reasons:       []string{"模板标记为必填销售属性"},
			}},
		},
		RequestDraft: &SheinRequestDraft{
			SKCList: []SheinSKCRequestDraft{{
				SupplierCode: "SKC-1",
				SkcName:      "Black",
				SaleAttribute: &SheinResolvedSaleAttribute{
					Scope:       "skc",
					Name:        "Color",
					Value:       "Black",
					AttributeID: 501,
				},
				ImageInfo: &SheinImageDraft{MainImage: "https://cdn.example.com/skc.jpg"},
				SKUList: []SheinSKUDraft{{
					SupplierSKU: "SKU-1",
					BasePrice:   "21.99",
					StockCount:  10,
					SaleAttributes: []SheinResolvedSaleAttribute{{
						Scope:            "sku",
						Name:             "Size",
						Value:            "L",
						AttributeID:      502,
						AttributeValueID: &skuValueID,
					}},
				}},
			}},
		},
	}

	context := buildSheinEditorContext(pkg)
	if context == nil {
		t.Fatal("expected editor context")
	}
	if context.Basics == nil || context.Basics.SpuName != "Bottle" {
		t.Fatalf("basics = %+v", context.Basics)
	}
	if context.RevisionSkeleton == nil || context.RevisionSkeleton.Platform != "shein" || context.RevisionSkeleton.Shein == nil {
		t.Fatalf("revision skeleton = %+v", context.RevisionSkeleton)
	}
	if context.RevisionSkeleton.Shein.ProductNameEn == nil || *context.RevisionSkeleton.Shein.ProductNameEn != "Bottle" {
		t.Fatalf("revision skeleton shein = %+v", context.RevisionSkeleton.Shein)
	}
	if context.DirtyHints == nil || len(context.DirtyHints.EditableFields) == 0 {
		t.Fatalf("dirty hints = %+v", context.DirtyHints)
	}
	if len(context.DirtyHints.DefaultChangedFields) == 0 {
		t.Fatalf("dirty hints default changed = %+v", context.DirtyHints)
	}
	if context.Progress == nil || context.Progress.Total == 0 || len(context.Progress.Sections) != 4 {
		t.Fatalf("progress = %+v", context.Progress)
	}
	if context.Category == nil || context.Category.SuggestedPatch == nil || context.Category.SuggestedPatch.CategoryID == nil || *context.Category.SuggestedPatch.CategoryID != 7788 {
		t.Fatalf("category context = %+v", context.Category)
	}
	if context.Category.Recommendation == nil || context.Category.Recommendation.Source != "suggest_category" || context.Category.Recommendation.Confidence != "high" {
		t.Fatalf("category recommendation = %+v", context.Category.Recommendation)
	}
	if len(context.Category.PreviewEffects) == 0 || len(context.Category.PreviewEffects[0].PreviewBlocks) == 0 {
		t.Fatalf("category preview effects = %+v", context.Category.PreviewEffects)
	}
	if context.Attributes == nil || context.Attributes.SuggestedPatch == nil || len(context.Attributes.SuggestedPatch.ResolvedAttributes) != 1 {
		t.Fatalf("attribute context = %+v", context.Attributes)
	}
	if len(context.Attributes.Suggestions) != 1 || context.Attributes.Suggestions[0].Confidence != "high" {
		t.Fatalf("attribute suggestions = %+v", context.Attributes.Suggestions)
	}
	if len(context.Attributes.PreviewEffects) == 0 || len(context.Attributes.PreviewEffects[0].AffectedFields) == 0 {
		t.Fatalf("attribute preview effects = %+v", context.Attributes.PreviewEffects)
	}
	if context.SaleAttributes == nil || context.SaleAttributes.SuggestedResolutionPatch == nil || context.SaleAttributes.SuggestedResolutionPatch.PrimaryAttributeID == nil {
		t.Fatalf("sale context = %+v", context.SaleAttributes)
	}
	if len(context.SaleAttributes.SuggestedSKCPatches) != 1 || len(context.SaleAttributes.SuggestedSKCPatches[0].SKUPatches) != 1 {
		t.Fatalf("sale patches = %+v", context.SaleAttributes.SuggestedSKCPatches)
	}
	if context.SaleAttributes.Recommendation == nil || context.SaleAttributes.Recommendation.Confidence != "high" {
		t.Fatalf("sale recommendation = %+v", context.SaleAttributes.Recommendation)
	}
	if len(context.SaleAttributes.CandidateSuggestions) != 1 || context.SaleAttributes.CandidateSuggestions[0].SelectedScope != "skc" {
		t.Fatalf("sale candidate suggestions = %+v", context.SaleAttributes.CandidateSuggestions)
	}
	if len(context.SaleAttributes.PreviewEffects) == 0 || len(context.SaleAttributes.PreviewEffects[0].PreviewBlocks) == 0 {
		t.Fatalf("sale preview effects = %+v", context.SaleAttributes.PreviewEffects)
	}
}

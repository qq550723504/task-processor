package workspace

import (
	"testing"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildSaleAttributePayloadIncludesCategoryReviewSignal(t *testing.T) {
	pkg := &sheinpub.Package{
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:                  "partial",
			Source:                  "sale_attribute_templates",
			RecommendCategoryReview: true,
			CategoryReviewReason:    "当前类目销售属性模板未提供可承接款式/型号的销售属性字段",
		},
	}

	payload := BuildSaleAttributePayload(pkg)
	if payload == nil {
		t.Fatal("expected payload")
	}
	if !payload.RecommendCategoryReview {
		t.Fatalf("recommend_category_review = %v, want true", payload.RecommendCategoryReview)
	}
	if payload.CategoryReviewReason != "当前类目销售属性模板未提供可承接款式/型号的销售属性字段" {
		t.Fatalf("category_review_reason = %q", payload.CategoryReviewReason)
	}
}

func TestBuildSaleAttributePayloadDowngradesResolvedStatusWhenValueIDsAreMissing(t *testing.T) {
	pkg := &sheinpub.Package{
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 1001466,
			SKCAttributes: []sheinpub.ResolvedSaleAttribute{{
				Scope:       "skc",
				Name:        "Plug(Voltage)",
				Value:       "white",
				AttributeID: 1001466,
			}},
		},
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{SupplierCode: "SKC-1"}},
		},
	}

	payload := BuildSaleAttributePayload(pkg)
	if payload == nil {
		t.Fatal("expected payload")
	}
	if payload.Status != "partial" {
		t.Fatalf("payload status = %q, want partial", payload.Status)
	}
}

func TestBuildSaleAttributePayloadIncludesManualReviewContext(t *testing.T) {
	pkg := &sheinpub.Package{
		SkcList: []sheinpub.SKCPackage{{
			SupplierCode: "SKC-1",
			Attributes: map[string]string{
				"Color": "White",
			},
		}},
		RequestDraft: &sheinpub.RequestDraft{
			SKCList: []sheinpub.SKCRequestDraft{{
				SupplierCode: "SKC-1",
				SkcName:      "White",
				SKUList: []sheinpub.SKUDraft{{
					SupplierSKU: "SKU-1",
					Attributes: map[string]string{
						"Size": "M",
					},
				}},
			}},
		},
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:                   "partial",
			PrimaryAttributeID:       27,
			SecondaryAttributeID:     87,
			PrimarySourceDimension:   "Color",
			SecondarySourceDimension: "Size",
			SourceDimensions: []sheinpub.SourceVariantDimension{{
				Name:          "Color",
				Values:        []string{"White", "Black"},
				DistinctCount: 2,
				SampleValue:   "White",
			}},
			TemplateOptions: []sheinpub.SaleAttributeTemplateOption{{
				AttributeID: 27,
				Name:        "Color",
				NameEn:      "Color",
				SKCScope:    true,
				AttributeValueList: []sheinpub.AttributeValueCandidate{{
					AttributeValueID: 112,
					Value:            "White",
					ValueEn:          "White",
				}},
			}},
		},
	}

	payload := BuildSaleAttributePayload(pkg)
	if payload == nil {
		t.Fatal("expected payload")
	}
	if payload.PrimarySourceDimension != "Color" {
		t.Fatalf("primary source dimension = %q", payload.PrimarySourceDimension)
	}
	if len(payload.TemplateOptions) != 1 || payload.TemplateOptions[0].AttributeID != 27 {
		t.Fatalf("template options = %#v", payload.TemplateOptions)
	}
	if len(payload.SKCPatches) != 1 || payload.SKCPatches[0].Attributes["Color"] != "White" {
		t.Fatalf("skc patches = %#v", payload.SKCPatches)
	}
}

func TestBuildAttributePayloadPrefersPendingAttributesFromResolution(t *testing.T) {
	pkg := &sheinpub.Package{
		ProductAttributes: []common.Attribute{{Name: "Material", Value: "Cotton"}},
		AttributeResolution: &sheinpub.AttributeResolution{
			Status:            "partial",
			PendingAttributes: []common.Attribute{{Name: "Width (cm)"}},
		},
	}

	payload := BuildAttributePayload(pkg)
	if payload == nil {
		t.Fatal("expected payload")
	}
	if len(payload.PendingAttributes) != 1 {
		t.Fatalf("pending attributes = %#v, want 1", payload.PendingAttributes)
	}
	if payload.PendingAttributes[0].Name != "Width (cm)" {
		t.Fatalf("pending attribute name = %q, want Width (cm)", payload.PendingAttributes[0].Name)
	}
}

func TestBuildAttributePayloadIncludesSizeChartAttributes(t *testing.T) {
	pkg := &sheinpub.Package{
		AttributeResolution: &sheinpub.AttributeResolution{
			Status: "resolved",
			SizeChartAttributes: []sheinpub.PendingAttributeCandidate{{
				AttributeID:     20,
				AttributeName:   "胸围 (cm)",
				AttributeNameEn: "Bust (cm)",
			}},
		},
	}

	payload := BuildAttributePayload(pkg)
	if payload == nil {
		t.Fatal("expected payload")
	}
	if len(payload.SizeChartAttributes) != 1 {
		t.Fatalf("size chart attributes = %#v, want 1", payload.SizeChartAttributes)
	}
	if payload.SizeChartAttributes[0].AttributeID != 20 {
		t.Fatalf("size chart attribute id = %d, want 20", payload.SizeChartAttributes[0].AttributeID)
	}
}

func TestBuildAttributePayloadDoesNotRecreatePendingAttributesAfterManualFallbackConfirmation(t *testing.T) {
	pkg := &sheinpub.Package{
		ProductAttributes: []common.Attribute{
			{Name: "material", Value: "涤纶"},
			{Name: "product_sku", Value: "MG8089002"},
		},
		AttributeResolution: &sheinpub.AttributeResolution{
			Status:            "resolved",
			Source:            "manual_fallback_review",
			ResolvedCount:     2,
			UnresolvedCount:   0,
			PendingAttributes: []common.Attribute{},
		},
	}

	payload := BuildAttributePayload(pkg)
	if payload == nil {
		t.Fatal("expected payload")
	}
	if len(payload.PendingAttributes) != 0 {
		t.Fatalf("pending attributes = %#v, want empty after fallback confirmation", payload.PendingAttributes)
	}
}

package shein

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildSaleAttributeResolutionPatchIncludesCategoryReviewSignal(t *testing.T) {
	pkg := &sheinpub.Package{
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:                  "partial",
			Source:                  "sale_attribute_templates",
			RecommendCategoryReview: true,
			CategoryReviewReason:    "当前类目销售属性模板未提供可承接款式/型号的销售属性字段",
			ReviewNotes:             []string{"note"},
		},
	}

	patch := BuildSaleAttributeResolutionPatch(pkg)
	if patch == nil {
		t.Fatal("expected patch")
	}
	if patch.RecommendCategoryReview == nil || !*patch.RecommendCategoryReview {
		t.Fatalf("recommend_category_review = %#v, want true", patch.RecommendCategoryReview)
	}
	if patch.CategoryReviewReason == nil || *patch.CategoryReviewReason != "当前类目销售属性模板未提供可承接款式/型号的销售属性字段" {
		t.Fatalf("category_review_reason = %#v", patch.CategoryReviewReason)
	}
}

func TestBuildSaleAttributeResolutionPatchDowngradesResolvedStatusWhenValueIDsAreMissing(t *testing.T) {
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

	patch := BuildSaleAttributeResolutionPatch(pkg)
	if patch == nil || patch.Status == nil {
		t.Fatalf("patch = %#v, want status", patch)
	}
	if *patch.Status != "partial" {
		t.Fatalf("patch status = %q, want partial", *patch.Status)
	}
}

func TestBuildSaleAttributeResolutionPatchIncludesValueAssignmentsAndSourceDimensions(t *testing.T) {
	skcValueID := 739
	skuValueID := 303468379
	pkg := &sheinpub.Package{
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			Status:                   "resolved",
			PrimaryAttributeID:       27,
			SecondaryAttributeID:     87,
			PrimarySourceDimension:   "Color",
			SecondarySourceDimension: "Size",
			SKCValueAssignments: map[string]sheinpub.ResolvedSaleAttribute{
				"white": {
					Scope:            "skc",
					Name:             "Color",
					Value:            "white",
					AttributeID:      27,
					AttributeValueID: &skcValueID,
				},
			},
			SKUValueAssignments: map[string]sheinpub.ResolvedSaleAttribute{
				`60×70.8inch (152×180cm)`: {
					Scope:            "sku",
					Name:             "Size",
					Value:            "60×70.8Inch (152×180cm)",
					AttributeID:      87,
					AttributeValueID: &skuValueID,
				},
			},
		},
	}

	patch := BuildSaleAttributeResolutionPatch(pkg)
	if patch == nil {
		t.Fatal("expected patch")
	}
	if patch.PrimarySourceDimension == nil || *patch.PrimarySourceDimension != "Color" {
		t.Fatalf("primary source dimension = %#v", patch.PrimarySourceDimension)
	}
	if patch.SecondarySourceDimension == nil || *patch.SecondarySourceDimension != "Size" {
		t.Fatalf("secondary source dimension = %#v", patch.SecondarySourceDimension)
	}
	if got := patch.SKCValueAssignments["white"].AttributeValueID; got == nil || *got != skcValueID {
		t.Fatalf("skc value assignments = %+v", patch.SKCValueAssignments)
	}
	if got := patch.SKUValueAssignments[`60×70.8inch (152×180cm)`].AttributeValueID; got == nil || *got != skuValueID {
		t.Fatalf("sku value assignments = %+v", patch.SKUValueAssignments)
	}
}

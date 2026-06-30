package shein

import (
	"testing"

	"task-processor/internal/catalog/canonical"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildPreviewProductIncludesSizeAttributeList(t *testing.T) {
	t.Parallel()

	valueID := 417
	pkg := &Package{
		DraftPayload: &RequestDraft{
			SpuName:      "Oversized Tee",
			SupplierCode: "SKU",
			SizeAttributeList: []sheinproduct.SizeAttribute{{
				AttributeID:                10,
				AttributeExtraValue:        "55",
				RelateSaleAttributeID:      87,
				RelateSaleAttributeValueID: valueID,
			}},
		},
	}

	got := BuildPreviewProduct(pkg)

	if got == nil {
		t.Fatal("BuildPreviewProduct() = nil")
	}
	if len(got.SizeAttributeList) != 1 {
		t.Fatalf("size_attribute_list = %#v, want 1 item", got.SizeAttributeList)
	}
	if got.SizeAttributeList[0].AttributeID != 10 || got.SizeAttributeList[0].RelateSaleAttributeValueID != valueID {
		t.Fatalf("size_attribute_list[0] = %#v", got.SizeAttributeList[0])
	}
}

func TestAssemblerBuildAppliesStructuredProductSizeToPreviewPayload(t *testing.T) {
	t.Parallel()

	sizeMValueID := 417
	sizeLValueID := 568
	assembler := NewAssembler(AssemblerConfig{
		SaleAttributeResolver: assemblerStubSaleAttributeResolver{
			resolution: &SaleAttributeResolution{
				Status:                   "resolved",
				SecondarySourceDimension: "Size",
				SecondaryAttributeID:     87,
				SKUValueAssignments: map[string]ResolvedSaleAttribute{
					normalizeText("M"): {Value: "M", AttributeID: 87, AttributeValueID: &sizeMValueID},
					normalizeText("L"): {Value: "L", AttributeID: 87, AttributeValueID: &sizeLValueID},
				},
			},
		},
	})
	canonicalProduct := &canonical.Product{
		Title: "Oversized Tee",
		Images: []canonical.Image{
			{URL: "https://example.com/main.jpg"},
		},
		Variants: []canonical.Variant{
			{
				SKU:        "SKU-M",
				Attributes: map[string]canonical.Attribute{"Size": {Value: "M"}},
				Stock:      5,
				IsDefault:  true,
			},
			{
				SKU:        "SKU-L",
				Attributes: map[string]canonical.Attribute{"Size": {Value: "L"}},
				Stock:      5,
			},
		},
	}
	productSize := `[[{"content":"尺码","remark":""},{"content":"肩宽(cm/in)","remark":""},{"content":"胸围(cm/in)","remark":""}],[{"content":"M","remark":""},{"content":"55cm/21.7in","remark":""},{"content":"112cm /44.1in","remark":""}],[{"content":"L","remark":""},{"content":"58cm/22.8in","remark":""},{"content":"118cm /46.5in","remark":""}]]`

	pkg := assembler.Build(&BuildRequest{Country: "US", Language: "en", ProductSize: productSize}, canonicalProduct, nil)

	if pkg == nil || pkg.PreviewPayload == nil {
		t.Fatal("expected preview payload")
	}
	got := pkg.PreviewPayload.SizeAttributeList
	if len(got) != 4 {
		t.Fatalf("size_attribute_list = %#v, want 4 items", got)
	}
	if got[0].AttributeID != 10 || got[0].AttributeExtraValue != "55" || got[0].RelateSaleAttributeID != 87 || got[0].RelateSaleAttributeValueID != sizeMValueID {
		t.Fatalf("first size attribute = %#v", got[0])
	}
	if got[3].AttributeID != 15 || got[3].AttributeExtraValue != "118" || got[3].RelateSaleAttributeValueID != sizeLValueID {
		t.Fatalf("last size attribute = %#v", got[3])
	}
}

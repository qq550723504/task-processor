package shein

import (
	"testing"

	"task-processor/internal/catalog/canonical"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildSizeAttributesFromStructuredProductSize(t *testing.T) {
	t.Parallel()

	sizeMValueID := 417
	sizeLValueID := 568
	pkg := &Package{
		SaleAttributeResolution: &SaleAttributeResolution{
			SecondaryAttributeID:     87,
			SecondarySourceDimension: "Size",
		},
		DraftPayload: &RequestDraft{
			SKCList: []SKCRequestDraft{{
				SKUList: []SKUDraft{
					{
						Attributes: map[string]string{"Size": "M"},
						SaleAttributes: []ResolvedSaleAttribute{{
							Value:            "M",
							AttributeID:      87,
							AttributeValueID: &sizeMValueID,
						}},
					},
					{
						Attributes: map[string]string{"Size": "L"},
						SaleAttributes: []ResolvedSaleAttribute{{
							Value:            "L",
							AttributeID:      87,
							AttributeValueID: &sizeLValueID,
						}},
					},
				},
			}},
		},
	}
	productSize := `[[{"content":"尺码","remark":""},{"content":"肩宽(cm/in)","remark":""},{"content":"胸围(cm/in)","remark":""},{"content":"衣长(cm/in)","remark":""},{"content":"袖长(cm/in)","remark":""}],[{"content":"M","remark":""},{"content":"55cm/21.7in","remark":""},{"content":"112cm /44.1in","remark":""},{"content":"71cm/28in","remark":""},{"content":"21.5cm/8.5in","remark":""}],[{"content":"L","remark":""},{"content":"58cm/22.8in","remark":""},{"content":"118cm /46.5in","remark":""},{"content":"74cm /29.1in","remark":""},{"content":"23cm /9.1in","remark":""}]]`

	got := buildSizeAttributesFromProductSize(productSize, pkg)

	want := []sheinproduct.SizeAttribute{
		{AttributeID: 10, AttributeExtraValue: "55", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 417},
		{AttributeID: 15, AttributeExtraValue: "112", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 417},
		{AttributeID: 20, AttributeExtraValue: "71", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 417},
		{AttributeID: 29, AttributeExtraValue: "21.5", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 417},
		{AttributeID: 10, AttributeExtraValue: "58", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 568},
		{AttributeID: 15, AttributeExtraValue: "118", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 568},
		{AttributeID: 20, AttributeExtraValue: "74", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 568},
		{AttributeID: 29, AttributeExtraValue: "23", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 568},
	}
	if len(got) != len(want) {
		t.Fatalf("size attributes = %#v, want %d items", got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("size attribute[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

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

package shein

import (
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestProductAttributesReadyForSubmit(t *testing.T) {
	valueID := 42
	tests := []struct {
		name  string
		attrs []ProductAttributeLike
		want  bool
	}{
		{
			name: "resolved attribute ids are ready",
			attrs: []ProductAttributeLike{{
				AttributeID:      1001,
				AttributeValueID: &valueID,
			}},
			want: true,
		},
		{
			name: "manual extra value is ready",
			attrs: []ProductAttributeLike{{
				AttributeID:         1002,
				AttributeExtraValue: "cotton",
			}},
			want: true,
		},
		{
			name: "missing attribute id is not ready",
			attrs: []ProductAttributeLike{{
				AttributeValueID: &valueID,
			}},
			want: false,
		},
		{
			name: "missing value is not ready",
			attrs: []ProductAttributeLike{{
				AttributeID: 1003,
			}},
			want: false,
		},
		{
			name: "empty list is not ready",
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			attrs := make([]sheinproduct.ProductAttribute, 0, len(tc.attrs))
			for _, item := range tc.attrs {
				attrs = append(attrs, item.toShein())
			}
			if got := ProductAttributesReadyForSubmit(attrs); got != tc.want {
				t.Fatalf("ProductAttributesReadyForSubmit() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRepairSubmitSaleAttributesRepairsDraftAndPreviewPayload(t *testing.T) {
	skcValueID := 101
	skuValueID := 202
	pkg := &Package{
		DraftPayload: &RequestDraft{
			SpuName:      "Test Dress",
			SupplierCode: "SPU-1",
			SKCList: []SKCRequestDraft{{
				SkcName:      "Red",
				SupplierCode: "SKC-RED",
				SKUList: []SKUDraft{{
					SupplierSKU: "SKU-RED-S",
				}},
			}},
		},
		SaleAttributeResolution: &SaleAttributeResolution{
			Status: "resolved",
			SKCAttributes: []ResolvedSaleAttribute{{
				AttributeID:      11,
				AttributeValueID: &skcValueID,
			}},
			SKUAttributes: []ResolvedSaleAttribute{{
				AttributeID:      22,
				AttributeValueID: &skuValueID,
			}},
		},
	}

	RepairSubmitSaleAttributes(pkg)

	skcAttr := pkg.DraftPayload.SKCList[0].SaleAttribute
	if skcAttr == nil || skcAttr.AttributeID != 11 || skcAttr.AttributeValueID == nil || *skcAttr.AttributeValueID != skcValueID {
		t.Fatalf("draft skc sale attribute = %#v, want resolved assignment", skcAttr)
	}
	skuAttrs := pkg.DraftPayload.SKCList[0].SKUList[0].SaleAttributes
	if len(skuAttrs) != 1 || skuAttrs[0].AttributeID != 22 || skuAttrs[0].AttributeValueID == nil || *skuAttrs[0].AttributeValueID != skuValueID {
		t.Fatalf("draft sku sale attributes = %#v, want resolved assignment", skuAttrs)
	}
	if pkg.PreviewPayload == nil {
		t.Fatal("PreviewPayload = nil, want rebuilt preview payload")
	}
	if len(pkg.PreviewPayload.SKCList) != 1 {
		t.Fatalf("preview skc count = %d, want 1", len(pkg.PreviewPayload.SKCList))
	}
	if got := pkg.PreviewPayload.SKCList[0].SaleAttribute.AttributeID; got != 11 {
		t.Fatalf("preview skc attribute id = %d, want 11", got)
	}
	if got := pkg.PreviewPayload.SKCList[0].SaleAttribute.AttributeValueID; got != skcValueID {
		t.Fatalf("preview skc attribute value id = %d, want %d", got, skcValueID)
	}
	if len(pkg.PreviewPayload.SKCList[0].SKUS) != 1 {
		t.Fatalf("preview sku count = %d, want 1", len(pkg.PreviewPayload.SKCList[0].SKUS))
	}
	if got := len(pkg.PreviewPayload.SKCList[0].SKUS[0].SaleAttributeList); got != 1 {
		t.Fatalf("preview sku sale attribute count = %d, want 1", got)
	}
	if got := pkg.PreviewPayload.SKCList[0].SKUS[0].SaleAttributeList[0].AttributeID; got != 22 {
		t.Fatalf("preview sku attribute id = %d, want 22", got)
	}
	if got := pkg.PreviewPayload.SKCList[0].SKUS[0].SaleAttributeList[0].AttributeValueID; got != skuValueID {
		t.Fatalf("preview sku attribute value id = %d, want %d", got, skuValueID)
	}
}

type ProductAttributeLike struct {
	AttributeID         int
	AttributeValueID    *int
	AttributeExtraValue string
}

func (a ProductAttributeLike) toShein() sheinproduct.ProductAttribute {
	return sheinproduct.ProductAttribute{
		AttributeID:         a.AttributeID,
		AttributeValueID:    a.AttributeValueID,
		AttributeExtraValue: a.AttributeExtraValue,
	}
}

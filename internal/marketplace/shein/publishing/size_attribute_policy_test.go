package publishing

import (
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildSizeAttributesFromStructuredProductSize(t *testing.T) {
	t.Parallel()

	productSize := `[[{"content":"尺码","remark":""},{"content":"肩宽(cm/in)","remark":""},{"content":"胸围(cm/in)","remark":""},{"content":"衣长(cm/in)","remark":""},{"content":"袖长(cm/in)","remark":""}],[{"content":"M","remark":""},{"content":"55cm/21.7in","remark":""},{"content":"112cm /44.1in","remark":""},{"content":"71cm/28in","remark":""},{"content":"21.5cm/8.5in","remark":""}],[{"content":"L","remark":""},{"content":"58cm/22.8in","remark":""},{"content":"118cm /46.5in","remark":""},{"content":"74cm /29.1in","remark":""},{"content":"23cm /9.1in","remark":""}]]`
	got := BuildSizeAttributesFromProductSize(productSize, []SizeSaleAttributeRef{
		{SizeValue: "M", AttributeID: 87, AttributeValueID: 417},
		{SizeValue: "L", AttributeID: 87, AttributeValueID: 568},
	})

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

func TestBuildSizeAttributesFromProductSizeUsesTemplateAttributeIDs(t *testing.T) {
	t.Parallel()

	productSize := `[[{"content":"尺码","remark":""},{"content":"胸围(cm/in)","remark":""},{"content":"衣长(cm/in)","remark":""}],[{"content":"M","remark":""},{"content":"112cm /44.1in","remark":""},{"content":"71cm/28in","remark":""}]]`
	got := BuildSizeAttributesFromProductSizeWithTemplates(productSize, []SizeSaleAttributeRef{
		{SizeValue: "M", AttributeID: 87, AttributeValueID: 417},
	}, []SizeChartTemplateAttribute{
		{AttributeID: 20, AttributeName: "胸围 (cm)", AttributeNameEn: "Bust (cm)"},
		{AttributeID: 55, AttributeName: "长度 (cm)", AttributeNameEn: "Length (cm)"},
	})

	want := []sheinproduct.SizeAttribute{
		{AttributeID: 20, AttributeExtraValue: "112", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 417},
		{AttributeID: 55, AttributeExtraValue: "71", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 417},
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

func TestBuildSizeAttributesFromProductSizeAcceptsHeaderUnitWithoutCellUnit(t *testing.T) {
	t.Parallel()

	productSize := `[[{"content":"尺码","remark":""},{"content":"衣长(cm/in)","remark":""},{"content":"胸围(cm/in)","remark":""}],[{"content":"S","remark":""},{"content":"87.5/34.45 ","remark":""},{"content":"87/34.25 ","remark":""}]]`
	got := BuildSizeAttributesFromProductSize(productSize, []SizeSaleAttributeRef{
		{SizeValue: "S", AttributeID: 87, AttributeValueID: 568},
	})

	want := []sheinproduct.SizeAttribute{
		{AttributeID: 20, AttributeExtraValue: "87.5", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 568},
		{AttributeID: 15, AttributeExtraValue: "87", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 568},
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

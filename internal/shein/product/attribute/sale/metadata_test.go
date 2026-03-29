package sale_test

import (
	"testing"

	sheinapi "task-processor/internal/shein/api/attribute"
	sheinctx "task-processor/internal/shein/context"
	sheinattr "task-processor/internal/shein/product/attribute"
	"task-processor/internal/shein/product/attribute/sale"
)

func newCalc() *sheinctx.AttributeImportanceCalculator {
	return sheinctx.NewAttributeImportanceCalculator()
}

func TestCalculateImportanceForSaleAttribute(t *testing.T) {
	calc := newCalc()

	tests := []struct {
		name string
		attr sheinapi.AttributeInfo
		want int
	}{
		{
			"all_zero",
			sheinapi.AttributeInfo{},
			0,
		},
		{
			"remark_list_adds_100",
			sheinapi.AttributeInfo{AttributeRemarkList: []any{"remark"}},
			100,
		},
		{
			"required_label_adds_80",
			sheinapi.AttributeInfo{AttributeLabel: 1},
			80,
		},
		{
			"sample_adds_40",
			sheinapi.AttributeInfo{IsSample: 1},
			40,
		},
		{
			"active_status_adds_30",
			sheinapi.AttributeInfo{AttributeStatus: 3},
			30,
		},
		{
			"display_adds_20",
			sheinapi.AttributeInfo{AttributeIsShow: 1},
			20,
		},
		{
			"combined_required_and_sample",
			sheinapi.AttributeInfo{AttributeLabel: 1, IsSample: 1},
			120, // 80 + 40
		},
		{
			"all_flags",
			sheinapi.AttributeInfo{
				AttributeRemarkList: []any{"r1", "r2"},
				AttributeLabel:      1,
				IsSample:            1,
				AttributeStatus:     3,
				AttributeIsShow:     1,
			},
			270, // 100+80+40+30+20
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sale.CalculateImportanceForSaleAttribute(calc, &tc.attr)
			if got != tc.want {
				t.Errorf("CalculateImportanceForSaleAttribute() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestSaleAttributeMetadataBuilder_BuildAttributeNameMappings(t *testing.T) {
	b := sale.NewSaleAttributeMetadataBuilder()

	templates := &sheinapi.AttributeTemplateInfo{
		Data: []sheinapi.AttributeTemplate{
			{
				AttributeInfos: []sheinapi.AttributeInfo{
					{AttributeID: 10, AttributeNameEn: "Color", AttributeName: "颜色"},
					{AttributeID: 20, AttributeNameEn: "Size", AttributeName: "尺寸"},
					{AttributeID: 30, AttributeNameEn: "", AttributeName: "材质"},
				},
			},
		},
	}

	attrData := sheinattr.BuildAttributeInfo{
		SaleAttributeData: []sheinattr.GenerateAttribute{
			{AttrID: 10},
			{AttrID: 20},
			{AttrID: 30},
			{AttrID: 99}, // 模板中不存在
		},
	}

	mappings := b.BuildAttributeNameMappings(attrData, templates)

	tests := []struct {
		attrID int
		want   string
	}{
		{10, "Color"},   // 优先英文名
		{20, "Size"},    // 优先英文名
		{30, "材质"},      // 无英文名，用中文名
		{99, "attr_99"}, // 模板中不存在，用 attr_ID 格式
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := mappings[tc.attrID]
			if got != tc.want {
				t.Errorf("mappings[%d] = %q, want %q", tc.attrID, got, tc.want)
			}
		})
	}
}

func TestSaleAttributeMetadataBuilder_BuildAttributeNameMappings_NilTemplates(t *testing.T) {
	b := sale.NewSaleAttributeMetadataBuilder()

	attrData := sheinattr.BuildAttributeInfo{
		SaleAttributeData: []sheinattr.GenerateAttribute{
			{AttrID: 10},
		},
	}

	mappings := b.BuildAttributeNameMappings(attrData, nil)
	if mappings[10] != "attr_10" {
		t.Errorf("expected fallback 'attr_10', got %q", mappings[10])
	}
}

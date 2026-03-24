package sale

import (
	"testing"

	sheinapi "task-processor/internal/shein/api/attribute"
)

func newSmartFilter() *SaleAttributeSmartFilter {
	return NewSaleAttributeSmartFilter()
}

func makeAttr(id int, nameEn string, attrType, label int) sheinapi.AttributeInfo {
	return sheinapi.AttributeInfo{
		AttributeID:     id,
		AttributeNameEn: nameEn,
		AttributeType:   attrType,
		AttributeLabel:  label,
	}
}

func TestSaleAttributeSmartFilter_getRequiredSaleAttributes(t *testing.T) {
	f := newSmartFilter()

	attrs := []sheinapi.AttributeInfo{
		makeAttr(1, "Color", 1, 1),    // 销售属性 + 必填
		makeAttr(2, "Size", 1, 0),     // 销售属性 + 非必填
		makeAttr(3, "Material", 0, 1), // 非销售属性 + 必填
		makeAttr(4, "Pattern", 1, 1),  // 销售属性 + 必填
	}

	result := f.getRequiredSaleAttributes(attrs)

	if len(result) != 2 {
		t.Fatalf("expected 2 required sale attributes, got %d", len(result))
	}
	if result[0].AttributeID != 1 {
		t.Errorf("first attr ID = %d, want 1", result[0].AttributeID)
	}
	if result[1].AttributeID != 4 {
		t.Errorf("second attr ID = %d, want 4", result[1].AttributeID)
	}
}

func TestSaleAttributeSmartFilter_getRequiredSaleAttributes_Empty(t *testing.T) {
	f := newSmartFilter()

	tests := []struct {
		name  string
		attrs []sheinapi.AttributeInfo
	}{
		{"nil_input", nil},
		{"no_sale_attrs", []sheinapi.AttributeInfo{makeAttr(1, "Material", 0, 1)}},
		{"sale_but_not_required", []sheinapi.AttributeInfo{makeAttr(1, "Color", 1, 0)}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := f.getRequiredSaleAttributes(tc.attrs)
			if len(result) != 0 {
				t.Errorf("expected empty, got %d", len(result))
			}
		})
	}
}

func TestSaleAttributeSmartFilter_countSaleAttributes(t *testing.T) {
	f := newSmartFilter()

	tests := []struct {
		name  string
		attrs []sheinapi.AttributeInfo
		want  int
	}{
		{
			"mixed_types",
			[]sheinapi.AttributeInfo{
				makeAttr(1, "Color", 1, 0),
				makeAttr(2, "Size", 1, 0),
				makeAttr(3, "Material", 0, 0),
				makeAttr(4, "Pattern", 1, 0),
			},
			3,
		},
		{"all_non_sale", []sheinapi.AttributeInfo{makeAttr(1, "Material", 0, 0)}, 0},
		{"empty", nil, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := f.countSaleAttributes(tc.attrs)
			if got != tc.want {
				t.Errorf("countSaleAttributes() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestSaleAttributeSmartFilter_isAttributeRelevant(t *testing.T) {
	f := newSmartFilter()

	tests := []struct {
		name     string
		attr     sheinapi.AttributeInfo
		analysis ProductVariationAnalysis
		want     bool
	}{
		{
			"required_attr_always_relevant",
			makeAttr(1, "Color", 1, 1),
			ProductVariationAnalysis{},
			true,
		},
		{
			"color_attr_with_color_variation",
			makeAttr(2, "Color Type", 1, 0),
			ProductVariationAnalysis{HasColorVariation: true},
			true,
		},
		{
			"color_attr_without_color_variation",
			makeAttr(2, "Color Type", 1, 0),
			ProductVariationAnalysis{HasColorVariation: false},
			false,
		},
		{
			"size_attr_with_size_variation",
			makeAttr(3, "Size", 1, 0),
			ProductVariationAnalysis{HasSizeVariation: true},
			true,
		},
		{
			"size_attr_without_size_variation",
			makeAttr(3, "Size", 1, 0),
			ProductVariationAnalysis{HasSizeVariation: false},
			false,
		},
		{
			"pattern_attr_with_pattern_variation",
			makeAttr(4, "Pattern Style", 1, 0),
			ProductVariationAnalysis{HasPatternVariation: true},
			true,
		},
		{
			"quantity_attr_with_quantity_variation",
			makeAttr(5, "Count", 1, 0),
			ProductVariationAnalysis{HasQuantityVariation: true},
			true,
		},
		{
			"other_attr_with_multiple_colors",
			makeAttr(6, "Occasion", 1, 0),
			ProductVariationAnalysis{UniqueColors: []string{"Red", "Blue"}},
			true,
		},
		{
			"other_attr_no_variation",
			makeAttr(6, "Occasion", 1, 0),
			ProductVariationAnalysis{},
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := f.isAttributeRelevant(tc.attr, tc.analysis)
			if got != tc.want {
				t.Errorf("isAttributeRelevant(%q) = %v, want %v", tc.attr.AttributeNameEn, got, tc.want)
			}
		})
	}
}

func TestSaleAttributeSmartFilter_selectDefaultSaleAttribute(t *testing.T) {
	f := newSmartFilter()

	t.Run("prefers_required_over_color", func(t *testing.T) {
		attrs := []sheinapi.AttributeInfo{
			makeAttr(1, "Color", 1, 0),
			makeAttr(2, "Pattern", 1, 1), // 必填
		}
		result := f.selectDefaultSaleAttribute(attrs)
		if result == nil || result.AttributeID != 2 {
			t.Errorf("expected required attr (ID=2), got %v", result)
		}
	})

	t.Run("prefers_color_over_size", func(t *testing.T) {
		attrs := []sheinapi.AttributeInfo{
			makeAttr(1, "Size", 1, 0),
			makeAttr(2, "Color", 1, 0),
		}
		result := f.selectDefaultSaleAttribute(attrs)
		if result == nil || result.AttributeID != 2 {
			t.Errorf("expected color attr (ID=2), got %v", result)
		}
	})

	t.Run("prefers_size_over_other", func(t *testing.T) {
		attrs := []sheinapi.AttributeInfo{
			makeAttr(1, "Occasion", 1, 0),
			makeAttr(2, "Size", 1, 0),
		}
		result := f.selectDefaultSaleAttribute(attrs)
		if result == nil || result.AttributeID != 2 {
			t.Errorf("expected size attr (ID=2), got %v", result)
		}
	})

	t.Run("returns_nil_when_no_sale_attrs", func(t *testing.T) {
		attrs := []sheinapi.AttributeInfo{
			makeAttr(1, "Material", 0, 0), // 非销售属性
		}
		result := f.selectDefaultSaleAttribute(attrs)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})

	t.Run("returns_nil_for_empty", func(t *testing.T) {
		result := f.selectDefaultSaleAttribute(nil)
		if result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})
}

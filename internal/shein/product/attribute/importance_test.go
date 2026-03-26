package attribute_test

import (
	"testing"

	sheinapi "task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/product/attribute"
)

func newImportanceService() *attribute.ImportanceService {
	return attribute.NewImportanceService()
}

func TestImportanceService_CalculateAttributeImportance(t *testing.T) {
	s := newImportanceService()

	tests := []struct {
		name       string
		attr       sheinapi.AttributeInfo
		wantMin    int // 期望分数下限
		wantFields struct {
			hasRemarkList     bool
			isLabelAttribute  bool
			isSampleAttribute bool
			isActiveStatus    bool
		}
	}{
		{
			name: "all_zero_no_score",
			attr: sheinapi.AttributeInfo{
				AttributeID:         1,
				AttributeNameEn:     "Weight",
				AttributeRemarkList: nil,
				AttributeLabel:      0,
				IsSample:            0,
				AttributeStatus:     0,
				AttributeIsShow:     0,
			},
			wantMin: 0,
		},
		{
			name: "label_attribute_adds_80",
			attr: sheinapi.AttributeInfo{
				AttributeID:     2,
				AttributeNameEn: "Brand",
				AttributeLabel:  1,
			},
			wantMin: 80,
			wantFields: struct {
				hasRemarkList     bool
				isLabelAttribute  bool
				isSampleAttribute bool
				isActiveStatus    bool
			}{isLabelAttribute: true},
		},
		{
			name: "sample_attribute_adds_40",
			attr: sheinapi.AttributeInfo{
				AttributeID:     3,
				AttributeNameEn: "Pattern",
				IsSample:        1,
			},
			wantMin: 40,
			wantFields: struct {
				hasRemarkList     bool
				isLabelAttribute  bool
				isSampleAttribute bool
				isActiveStatus    bool
			}{isSampleAttribute: true},
		},
		{
			name: "active_status_adds_30",
			attr: sheinapi.AttributeInfo{
				AttributeID:     4,
				AttributeNameEn: "Occasion",
				AttributeStatus: 3,
			},
			wantMin: 30,
			wantFields: struct {
				hasRemarkList     bool
				isLabelAttribute  bool
				isSampleAttribute bool
				isActiveStatus    bool
			}{isActiveStatus: true},
		},
		{
			name: "key_primary_color_adds_60",
			attr: sheinapi.AttributeInfo{
				AttributeID:     5,
				AttributeNameEn: "color",
			},
			wantMin: 60, // IsKeyPrimaryAttribute 加 60
		},
		{
			name: "combined_label_and_sample",
			attr: sheinapi.AttributeInfo{
				AttributeID:     6,
				AttributeNameEn: "Style",
				AttributeLabel:  1,
				IsSample:        1,
			},
			wantMin: 120, // 80 + 40
			wantFields: struct {
				hasRemarkList     bool
				isLabelAttribute  bool
				isSampleAttribute bool
				isActiveStatus    bool
			}{isLabelAttribute: true, isSampleAttribute: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := s.CalculateAttributeImportance(&tc.attr)
			if result.Importance < tc.wantMin {
				t.Errorf("Importance = %d, want >= %d", result.Importance, tc.wantMin)
			}
			if result.IsLabelAttribute != tc.wantFields.isLabelAttribute {
				t.Errorf("IsLabelAttribute = %v, want %v", result.IsLabelAttribute, tc.wantFields.isLabelAttribute)
			}
			if result.IsSampleAttribute != tc.wantFields.isSampleAttribute {
				t.Errorf("IsSampleAttribute = %v, want %v", result.IsSampleAttribute, tc.wantFields.isSampleAttribute)
			}
			if result.IsActiveStatus != tc.wantFields.isActiveStatus {
				t.Errorf("IsActiveStatus = %v, want %v", result.IsActiveStatus, tc.wantFields.isActiveStatus)
			}
		})
	}
}

func TestImportanceService_IsKeyPrimaryAttribute(t *testing.T) {
	s := newImportanceService()

	tests := []struct {
		name       string
		attrName   string
		attrNameEn string
		want       bool
	}{
		{"chinese_color", "颜色", "", true},
		{"english_color", "", "color", true},
		{"english_material", "", "material", true},
		{"english_scent", "", "scent", true},
		{"english_function", "", "function", true},
		{"chinese_size_is_false", "尺寸", "", false},
		{"english_size_is_false", "", "size", false},
		{"unknown_attribute", "重量", "weight", false},
		{"contains_color_keyword", "颜色类型", "", true},
		{"contains_material_keyword", "", "Material Type", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := s.IsKeyPrimaryAttribute(tc.attrName, tc.attrNameEn)
			if got != tc.want {
				t.Errorf("IsKeyPrimaryAttribute(%q, %q) = %v, want %v",
					tc.attrName, tc.attrNameEn, got, tc.want)
			}
		})
	}
}

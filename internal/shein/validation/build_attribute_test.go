package validation_test

import (
	"testing"

	sheinapi "task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/validation"
)

func newAttributeValidator() *validation.AttributeValidator {
	return validation.NewAttributeValidator()
}

func makeAttrInfo(remarkList []any, label, status, isShow int) sheinapi.AttributeInfo {
	return sheinapi.AttributeInfo{
		AttributeRemarkList: remarkList,
		AttributeLabel:      label,
		AttributeStatus:     status,
		AttributeIsShow:     isShow,
	}
}

func TestAttributeValidator_IsAttributeRequired(t *testing.T) {
	v := newAttributeValidator()

	tests := []struct {
		name string
		attr sheinapi.AttributeInfo
		want bool
	}{
		{
			"remark_list_makes_required",
			makeAttrInfo([]any{"remark"}, 0, 0, 0),
			true,
		},
		{
			"label_1_makes_required",
			makeAttrInfo(nil, 1, 0, 0),
			true,
		},
		{
			"status_3_makes_required",
			makeAttrInfo(nil, 0, 3, 0),
			true,
		},
		{
			"is_show_1_not_required",
			makeAttrInfo(nil, 0, 0, 1),
			false,
		},
		{
			"all_zero_not_required",
			makeAttrInfo(nil, 0, 0, 0),
			false,
		},
		{
			"remark_takes_priority_over_others",
			makeAttrInfo([]any{"r"}, 1, 3, 1),
			true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := v.IsAttributeRequired(tc.attr)
			if got != tc.want {
				t.Errorf("IsAttributeRequired() = %v, want %v", got, tc.want)
			}
		})
	}
}

func makeAttrWithValues(nameEn string, values []sheinapi.AttributeValue) sheinapi.AttributeInfo {
	return sheinapi.AttributeInfo{
		AttributeNameEn:        nameEn,
		AttributeValueInfoList: values,
	}
}

func makeValues(n int) []sheinapi.AttributeValue {
	vals := make([]sheinapi.AttributeValue, n)
	for i := range vals {
		vals[i] = sheinapi.AttributeValue{AttributeValueEn: "val"}
	}
	return vals
}

func TestAttributeValidator_IsSaleSpecRequired(t *testing.T) {
	v := newAttributeValidator()

	tests := []struct {
		name string
		attr sheinapi.AttributeInfo
		want bool
	}{
		{
			"color_with_multiple_values",
			makeAttrWithValues("Color Type", makeValues(3)),
			true,
		},
		{
			"colour_with_multiple_values",
			makeAttrWithValues("Colour", makeValues(2)),
			true,
		},
		{
			"size_with_multiple_values",
			makeAttrWithValues("Size", makeValues(5)),
			true,
		},
		{
			"color_with_single_value",
			makeAttrWithValues("Color", makeValues(1)),
			false,
		},
		{
			"non_core_attribute_with_multiple_values",
			makeAttrWithValues("Material", makeValues(3)),
			false,
		},
		{
			"color_with_no_values",
			makeAttrWithValues("Color", nil),
			false,
		},
		{
			"empty_name_with_values",
			makeAttrWithValues("", makeValues(3)),
			false,
		},
		{
			"case_insensitive_COLOR",
			makeAttrWithValues("COLOR", makeValues(2)),
			true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := v.IsSaleSpecRequired(tc.attr)
			if got != tc.want {
				t.Errorf("IsSaleSpecRequired(%q, values=%d) = %v, want %v",
					tc.attr.AttributeNameEn, len(tc.attr.AttributeValueInfoList), got, tc.want)
			}
		})
	}
}

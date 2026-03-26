package build_test

import (
	"testing"

	sheinapi "task-processor/internal/shein/api/attribute"
	"task-processor/internal/shein/product/build"
	"task-processor/internal/shein/validation"
)

func newBuilder() *build.AttributeBuilder {
	return build.NewAttributeBuilder(validation.NewAttributeValidator())
}

func TestAttributeBuilder_BuildAttributeValues(t *testing.T) {
	b := newBuilder()

	tests := []struct {
		name      string
		input     []sheinapi.AttributeValue
		wantLen   int
		wantFirst struct {
			id    int
			value string
		}
	}{
		{
			"empty_list",
			nil,
			0,
			struct {
				id    int
				value string
			}{},
		},
		{
			"single_value",
			[]sheinapi.AttributeValue{
				{AttributeValueID: 101, AttributeValueEn: "Black"},
			},
			1,
			struct {
				id    int
				value string
			}{101, "Black"},
		},
		{
			"multiple_values",
			[]sheinapi.AttributeValue{
				{AttributeValueID: 101, AttributeValueEn: "Black"},
				{AttributeValueID: 102, AttributeValueEn: "White"},
				{AttributeValueID: 103, AttributeValueEn: "Red"},
			},
			3,
			struct {
				id    int
				value string
			}{101, "Black"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := b.BuildAttributeValues(tc.input)
			if len(got) != tc.wantLen {
				t.Fatalf("len = %d, want %d", len(got), tc.wantLen)
			}
			if tc.wantLen > 0 {
				if got[0].ID != tc.wantFirst.id {
					t.Errorf("got[0].ID = %d, want %d", got[0].ID, tc.wantFirst.id)
				}
				if got[0].Value != tc.wantFirst.value {
					t.Errorf("got[0].Value = %q, want %q", got[0].Value, tc.wantFirst.value)
				}
			}
		})
	}
}

func TestAttributeBuilder_BuildGenerateAttribute(t *testing.T) {
	b := newBuilder()

	tests := []struct {
		name         string
		attr         sheinapi.AttributeInfo
		wantRequired bool
		wantAttrID   int
		wantType     int
	}{
		{
			"required_by_label",
			sheinapi.AttributeInfo{
				AttributeID:    10,
				AttributeLabel: 1,
				AttributeMode:  2,
				AttributeValueInfoList: []sheinapi.AttributeValue{
					{AttributeValueID: 1, AttributeValueEn: "val"},
				},
			},
			true, 10, 2,
		},
		{
			"not_required",
			sheinapi.AttributeInfo{
				AttributeID:   20,
				AttributeMode: 1,
			},
			false, 20, 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := b.BuildGenerateAttribute(tc.attr)
			if got.AttrID != tc.wantAttrID {
				t.Errorf("AttrID = %d, want %d", got.AttrID, tc.wantAttrID)
			}
			if got.Required != tc.wantRequired {
				t.Errorf("Required = %v, want %v", got.Required, tc.wantRequired)
			}
			if got.Type != tc.wantType {
				t.Errorf("Type = %d, want %d", got.Type, tc.wantType)
			}
		})
	}
}

func TestAttributeBuilder_BuildSaleGenerateAttribute(t *testing.T) {
	b := newBuilder()

	tests := []struct {
		name         string
		attr         sheinapi.AttributeInfo
		wantRequired bool
	}{
		{
			"color_with_multiple_values_is_required",
			sheinapi.AttributeInfo{
				AttributeID:     30,
				AttributeNameEn: "Color",
				AttributeMode:   1,
				AttributeValueInfoList: []sheinapi.AttributeValue{
					{AttributeValueID: 1, AttributeValueEn: "Black"},
					{AttributeValueID: 2, AttributeValueEn: "White"},
				},
			},
			true,
		},
		{
			"material_not_required",
			sheinapi.AttributeInfo{
				AttributeID:     40,
				AttributeNameEn: "Material",
				AttributeMode:   1,
				AttributeValueInfoList: []sheinapi.AttributeValue{
					{AttributeValueID: 1, AttributeValueEn: "Cotton"},
					{AttributeValueID: 2, AttributeValueEn: "Polyester"},
				},
			},
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := b.BuildSaleGenerateAttribute(tc.attr)
			if got.Required != tc.wantRequired {
				t.Errorf("Required = %v, want %v", got.Required, tc.wantRequired)
			}
			if got.AttrID != tc.attr.AttributeID {
				t.Errorf("AttrID = %d, want %d", got.AttrID, tc.attr.AttributeID)
			}
		})
	}
}

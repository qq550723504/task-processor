package shein

import (
	"strings"
	"testing"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

func TestClassifyDisplayTemplateAttribute(t *testing.T) {
	tests := []struct {
		name string
		attr sheinattribute.AttributeInfo
		want string
	}{
		{
			name: "general attribute",
			attr: sheinattribute.AttributeInfo{AttributeType: 4},
			want: displayAttributeKindGeneral,
		},
		{
			name: "numeric attribute",
			attr: sheinattribute.AttributeInfo{AttributeType: 2},
			want: displayAttributeKindNumeric,
		},
		{
			name: "composition attribute",
			attr: sheinattribute.AttributeInfo{AttributeType: 3},
			want: displayAttributeKindComposition,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyDisplayTemplateAttribute(tc.attr)
			if got.Kind != tc.want {
				t.Fatalf("kind = %q, want %q", got.Kind, tc.want)
			}
		})
	}
}

func TestMatchTemplateAttributeValueReturnsNumericFallbackWithTemplateMetadata(t *testing.T) {
	attr := sheinattribute.AttributeInfo{
		AttributeID:        118,
		AttributeNameEn:    "Width (cm)",
		AttributeType:      2,
		AttributeMode:      0,
		DataDimension:      1,
		AttributeInputNum:  1,
		CascadeAttributeID: 1001,
	}

	resolved, notes := matchTemplateAttributeValue(attr, "Width", "120", nil, nil)
	if resolved.AttributeID != 118 {
		t.Fatalf("attribute id = %d, want 118", resolved.AttributeID)
	}
	if resolved.AttributeExtraValue != "120" {
		t.Fatalf("attribute extra value = %q, want 120", resolved.AttributeExtraValue)
	}
	if resolved.AttributeType != 2 || resolved.AttributeMode != 0 || resolved.DataDimension != 1 {
		t.Fatalf("unexpected template metadata: %+v", resolved)
	}
	if resolved.CascadeAttributeID != 1001 {
		t.Fatalf("cascade attribute id = %d, want 1001", resolved.CascadeAttributeID)
	}
	if len(notes) == 0 {
		t.Fatalf("expected numeric notes, got none")
	}
}

func TestNewDisplayTemplateIndexSkipsSaleAttributes(t *testing.T) {
	idx := newDisplayTemplateIndex([]sheinattribute.AttributeInfo{
		{
			AttributeID:     27,
			AttributeNameEn: "Color",
			AttributeType:   1,
			SKCScope:        boolPointer(true),
		},
		{
			AttributeID:     118,
			AttributeNameEn: "Width (cm)",
			AttributeType:   2,
		},
	})

	if attr := idx.FindAttribute("Color"); attr != nil {
		t.Fatalf("expected sale attribute to be skipped, got %+v", attr)
	}
	if attr := idx.FindAttribute("Width (cm)"); attr == nil || attr.AttributeID != 118 {
		t.Fatalf("expected display attribute 118, got %+v", attr)
	}
}

func TestNewDisplayTemplateIndexSkipsGarmentSizeChartAttributes(t *testing.T) {
	idx := newDisplayTemplateIndex([]sheinattribute.AttributeInfo{
		{
			AttributeID:        20,
			AttributeName:      "胸围 (cm)",
			AttributeNameEn:    "Bust (cm)",
			AttributeType:      2,
			AttributeMode:      0,
			DataDimension:      2,
			SourceSystemIDList: []int{1, 2, 6, 7},
		},
		{
			AttributeID:     160,
			AttributeNameEn: "Material",
			AttributeType:   4,
		},
	})

	if attr := idx.FindAttribute("Bust (cm)"); attr != nil {
		t.Fatalf("expected size chart attribute to be skipped, got %+v", attr)
	}
	if attr := idx.FindAttribute("Material"); attr == nil || attr.AttributeID != 160 {
		t.Fatalf("expected material display attribute, got %+v", attr)
	}
}

func TestNewDisplayTemplateIndexSkipsGarmentSizeChartAttributesWithoutLegacySourceSignature(t *testing.T) {
	idx := newDisplayTemplateIndex([]sheinattribute.AttributeInfo{
		{
			AttributeID:     55,
			AttributeName:   "长度 (cm)",
			AttributeNameEn: "Length (cm)",
			AttributeType:   2,
			AttributeMode:   0,
			DataDimension:   2,
		},
		{
			AttributeID:     20,
			AttributeName:   "胸围 (cm)",
			AttributeNameEn: "Bust (cm)",
			AttributeType:   2,
			AttributeMode:   0,
			DataDimension:   2,
		},
	})

	if attr := idx.FindAttribute("Length (cm)"); attr != nil {
		t.Fatalf("expected length size chart attribute to be skipped, got %+v", attr)
	}
	if attr := idx.FindAttribute("Bust (cm)"); attr != nil {
		t.Fatalf("expected bust size chart attribute to be skipped, got %+v", attr)
	}
}

func TestNewDisplayTemplateIndexKeepsNumericAttributesWithoutSizeChartSignature(t *testing.T) {
	idx := newDisplayTemplateIndex([]sheinattribute.AttributeInfo{
		{
			AttributeID:        118,
			AttributeName:      "宽度 (cm)",
			AttributeNameEn:    "Width (cm)",
			AttributeType:      2,
			AttributeMode:      0,
			DataDimension:      2,
			SourceSystemIDList: []int{1, 2, 3, 4, 6, 7},
		},
	})

	if attr := idx.FindAttribute("Width (cm)"); attr == nil || attr.AttributeID != 118 {
		t.Fatalf("expected non-size-chart numeric attribute, got %+v", attr)
	}
}

func TestBuildAttributeInputsIncludesDerivedDimensionValues(t *testing.T) {
	pkg := &Package{
		RequestDraft: &RequestDraft{
			SKCList: []SKCRequestDraft{{
				SKUList: []SKUDraft{{
					Length: "150",
					Width:  "100",
					Height: "10",
				}},
			}},
		},
	}

	inputs := buildAttributeInputs(pkg)
	found := map[string]string{}
	for _, item := range inputs {
		found[item.Name] = item.Value
	}
	if found["Length (cm)"] != "150" {
		t.Fatalf("length input = %q, want 150", found["Length (cm)"])
	}
	if found["Width (cm)"] != "100" {
		t.Fatalf("width input = %q, want 100", found["Width (cm)"])
	}
	if found["Height (cm)"] != "10" {
		t.Fatalf("height input = %q, want 10", found["Height (cm)"])
	}
}

func TestCompositionAttributeNotesDetectsInvalidPercentTotal(t *testing.T) {
	attr := sheinattribute.AttributeInfo{
		AttributeID:       62,
		AttributeNameEn:   "Composition",
		AttributeType:     3,
		AttributeInputNum: 1,
	}

	notes := compositionAttributeNotes(attr, "Polyester 80%, Cotton 10%")
	found := false
	for _, note := range notes {
		if strings.Contains(note, "100%") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected percentage consistency note, got %v", notes)
	}
}

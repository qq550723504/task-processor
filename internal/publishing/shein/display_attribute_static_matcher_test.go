package shein

import (
	"testing"

	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func TestBuildDerivedAttributeInputsIncludesProductContextWithoutSKU(t *testing.T) {
	inputs := buildDerivedAttributeInputs(&Package{
		SpuName:      "带刻度方形挂钟25*25",
		CategoryPath: []string{"家居&生活", "家居装饰", "时钟", "挂钟"},
		Description:  "Battery powered wall clock",
	})
	got := map[string]string{}
	for _, input := range inputs {
		got[input.Name] = input.Value
	}
	if got["Product Title"] == "" || got["Category Path"] == "" || got["Description"] == "" {
		t.Fatalf("derived inputs = %#v, want product title, category path and description", inputs)
	}
}

func TestImportantTemplateAttributesBecomePendingCandidates(t *testing.T) {
	doc := "Important marketplace attribute"
	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:       9001,
			AttributeNameEn:   "Product Model",
			AttributeLabel:    1,
			AttributeInputNum: 0,
			AttributeDoc:      &doc,
		},
		{
			AttributeID:       9002,
			AttributeNameEn:   "Optional Note",
			AttributeInputNum: 0,
		},
	}

	candidates := buildPendingAttributeCandidates(attributes, nil, []common.Attribute{{Name: "sku", Value: "MG17701062"}})
	if len(candidates) != 1 {
		t.Fatalf("candidates = %#v, want one important template candidate", candidates)
	}
	if !candidates[0].Important || candidates[0].Required {
		t.Fatalf("candidate flags = required:%t important:%t, want important only", candidates[0].Required, candidates[0].Important)
	}
	if candidates[0].Name != "Product Model" {
		t.Fatalf("candidate name = %q, want Product Model", candidates[0].Name)
	}
}

func TestOptionalTemplateAttributesBecomeRecommendedCandidates(t *testing.T) {
	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:     9001,
			AttributeNameEn: "Product Model",
			AttributeLabel:  1,
		},
		{
			AttributeID:     9002,
			AttributeNameEn: "Optional Note",
		},
	}

	candidates := buildRecommendedAttributeCandidates(attributes, nil, []common.Attribute{{Name: "sku", Value: "MG17701062"}})
	if len(candidates) != 1 {
		t.Fatalf("recommended candidates = %#v, want one optional template candidate", candidates)
	}
	if candidates[0].Required || candidates[0].Important {
		t.Fatalf("candidate flags = required:%t important:%t, want optional only", candidates[0].Required, candidates[0].Important)
	}
	if candidates[0].Name != "Optional Note" {
		t.Fatalf("candidate name = %q, want Optional Note", candidates[0].Name)
	}
}

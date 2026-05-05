package shein

import (
	"testing"

	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func TestMatchTemplateAttributeValueExactDoesNotCallLLM(t *testing.T) {
	t.Parallel()

	attr := sheinattribute.AttributeInfo{
		AttributeID:     160,
		AttributeNameEn: "Material",
		AttributeMode:   1,
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 526, AttributeValueEn: "Polyester", AttributeValue: "聚酯纤维(涤纶)"},
		},
	}

	resolved, _ := matchTemplateAttributeValue(
		attr,
		"Material",
		"Polyester",
		[]common.Attribute{{Name: "Material", Value: "Polyester"}},
		panicDisplayAttributeLLM{},
	)
	if resolved.AttributeID != 160 {
		t.Fatalf("attribute id = %d, want 160", resolved.AttributeID)
	}
	if resolved.AttributeValueID == nil || *resolved.AttributeValueID != 526 {
		t.Fatalf("attribute value id = %v, want 526", resolved.AttributeValueID)
	}
	if resolved.MatchedBy != "attribute_value" {
		t.Fatalf("matched by = %q, want attribute_value", resolved.MatchedBy)
	}
}

func TestMatchTemplateAttributeValueLegacyMatcherDoesNotCallLLM(t *testing.T) {
	t.Parallel()

	attr := sheinattribute.AttributeInfo{
		AttributeID:     160,
		AttributeNameEn: "Material",
		AttributeMode:   1,
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 526, AttributeValueEn: "Polyester", AttributeValue: "聚酯纤维(涤纶)"},
		},
	}

	resolved, _ := matchTemplateAttributeValue(
		attr,
		"Material",
		"Polyester!",
		[]common.Attribute{{Name: "Material", Value: "Polyester!"}},
		panicDisplayAttributeLLM{},
	)
	if resolved.AttributeID != 160 {
		t.Fatalf("attribute id = %d, want 160", resolved.AttributeID)
	}
	if resolved.AttributeValueID == nil || *resolved.AttributeValueID != 526 {
		t.Fatalf("attribute value id = %v, want 526", resolved.AttributeValueID)
	}
	if resolved.MatchedBy != "attribute_value_legacy_matcher" {
		t.Fatalf("matched by = %q, want attribute_value_legacy_matcher", resolved.MatchedBy)
	}
}

func TestMatchTemplateAttributeValuesBatchResolvesMultipleAttributes(t *testing.T) {
	t.Parallel()

	llm := &captureAttributeLLM{
		responses: []string{
			`{"selections":[{"attribute_id":160,"attribute_value_id":526,"reasons":["polyester aligns with material candidate"]},{"attribute_id":161,"attribute_value_id":701,"reasons":["outdoor aligns with room candidate"]}]}`,
		},
	}
	entries := []unresolvedDisplayAttributeValue{
		{
			Source: common.Attribute{Name: "Material", Value: "Polyester"},
			Attr: sheinattribute.AttributeInfo{
				AttributeID:     160,
				AttributeNameEn: "Material",
				AttributeMode:   1,
				AttributeValueInfoList: []sheinattribute.AttributeValue{
					{AttributeValueID: 526, AttributeValueEn: "Polyester", AttributeValue: "聚酯纤维(涤纶)"},
				},
			},
		},
		{
			Source: common.Attribute{Name: "Room Hint", Value: "Outdoor"},
			Attr: sheinattribute.AttributeInfo{
				AttributeID:     161,
				AttributeNameEn: "Room",
				AttributeMode:   1,
				AttributeValueInfoList: []sheinattribute.AttributeValue{
					{AttributeValueID: 701, AttributeValueEn: "Outdoor", AttributeValue: "户外"},
				},
			},
		},
	}

	resolved, notes := matchTemplateAttributeValuesBatch(entries, []common.Attribute{{Name: "Scene", Value: "Outdoor"}}, llm)
	if len(resolved) != 2 {
		t.Fatalf("resolved = %#v, want 2", resolved)
	}
	if got := resolved[160].AttributeValueID; got == nil || *got != 526 {
		t.Fatalf("material attribute value id = %v, want 526", got)
	}
	if resolved[160].MatchedBy != "llm_attribute_value_batch" {
		t.Fatalf("material matched by = %q, want llm_attribute_value_batch", resolved[160].MatchedBy)
	}
	if got := resolved[161].AttributeValueID; got == nil || *got != 701 {
		t.Fatalf("room attribute value id = %v, want 701", got)
	}
	if len(notes) < 2 {
		t.Fatalf("notes = %#v, want batch reasons", notes)
	}
}

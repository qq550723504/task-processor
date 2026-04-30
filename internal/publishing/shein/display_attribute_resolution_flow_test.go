package shein

import (
	"testing"

	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func TestResolveDisplayAttributesSkipsDuplicateResolvedAttributeIDs(t *testing.T) {
	t.Parallel()

	attributes := []sheinattribute.AttributeInfo{{
		AttributeID:     160,
		AttributeNameEn: "Material",
		AttributeMode:   1,
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 526, AttributeValueEn: "Polyester", AttributeValue: "聚酯纤维(涤纶)"},
		},
	}}
	inputs := []common.Attribute{
		{Name: "material", Value: "Polyester"},
		{Name: "Description", Value: "Polyester"},
	}

	resolved, _, _, _, _ := resolveDisplayAttributes(attributes, inputs, nil)
	if len(resolved) != 1 {
		t.Fatalf("resolved attributes = %#v, want exactly 1 material attribute", resolved)
	}
	if resolved[0].AttributeID != 160 {
		t.Fatalf("attribute id = %d, want 160", resolved[0].AttributeID)
	}
}

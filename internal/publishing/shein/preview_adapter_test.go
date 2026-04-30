package shein

import "testing"

func TestToResolvedAttributesSetsSingleCompositionPercentToHundred(t *testing.T) {
	t.Parallel()

	valueID := 526
	materialID := 160
	pkg := &Package{
		ResolvedAttributes: []ResolvedAttribute{
			{
				Name:             "Composition",
				Value:            "Polyester",
				AttributeID:      62,
				AttributeValueID: &valueID,
				AttributeType:    3,
			},
			{
				Name:             "Material",
				Value:            "Polyester",
				AttributeID:      materialID,
				AttributeValueID: &valueID,
				AttributeType:    4,
			},
			{
				Name:             "Material",
				Value:            "Polyester from description",
				AttributeID:      materialID,
				AttributeValueID: &valueID,
				AttributeType:    4,
			},
		},
		RequestDraft: &RequestDraft{},
	}

	got := toResolvedAttributes(pkg)
	if len(got) != 2 {
		t.Fatalf("product attributes = %#v, want 2 unique attributes", got)
	}
	if got[0].AttributeExtraValue != "100" {
		t.Fatalf("composition attribute_extra_value = %q, want 100", got[0].AttributeExtraValue)
	}
}

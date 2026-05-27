package shein

import (
	"testing"

	common "task-processor/internal/publishing/common"
)

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

func TestToResolvedAttributesSkipsNumericDisplayAttributes(t *testing.T) {
	t.Parallel()

	widthID := 118
	materialID := 160
	valueID := 526
	pkg := &Package{
		ResolvedAttributes: []ResolvedAttribute{
			{
				Name:                "Width (cm)",
				Value:               "25",
				AttributeID:         widthID,
				AttributeExtraValue: "25",
				AttributeType:       2,
			},
			{
				Name:             "Material",
				Value:            "Polyester",
				AttributeID:      materialID,
				AttributeValueID: &valueID,
				AttributeType:    4,
			},
		},
		RequestDraft: &RequestDraft{},
	}

	got := toResolvedAttributes(pkg)
	if len(got) != 1 {
		t.Fatalf("product attributes = %#v, want only non-numeric attributes", got)
	}
	if got[0].AttributeID != materialID {
		t.Fatalf("attribute id = %d, want %d", got[0].AttributeID, materialID)
	}
}

func TestToResolvedAttributesInfersQuantityExtraValueFromProductTitle(t *testing.T) {
	t.Parallel()

	valueID := 1002451
	pkg := &Package{
		ProductNameEn: "3-Piece Framed Canvas Wall Art Set",
		ResolvedAttributes: []ResolvedAttribute{
			{
				Name:             "Quantity",
				Value:            "piece(s)",
				AttributeID:      1000411,
				AttributeValueID: &valueID,
				AttributeType:    4,
			},
		},
		RequestDraft: &RequestDraft{},
	}

	got := toResolvedAttributes(pkg)
	if len(got) != 1 {
		t.Fatalf("product attributes = %#v, want 1 attribute", got)
	}
	if got[0].AttributeExtraValue != "3" {
		t.Fatalf("quantity attribute_extra_value = %q, want 3", got[0].AttributeExtraValue)
	}
}

func TestToResolvedAttributesInfersQuantityExtraValueFromProductAttributes(t *testing.T) {
	t.Parallel()

	valueID := 1002451
	pkg := &Package{
		ProductAttributes: []common.Attribute{
			{Name: "package_quantity", Value: "Pack of 5"},
		},
		ResolvedAttributes: []ResolvedAttribute{
			{
				Name:             "Quantity",
				Value:            "piece(s)",
				AttributeID:      1000411,
				AttributeValueID: &valueID,
				AttributeType:    4,
			},
		},
		RequestDraft: &RequestDraft{},
	}

	got := toResolvedAttributes(pkg)
	if len(got) != 1 {
		t.Fatalf("product attributes = %#v, want 1 attribute", got)
	}
	if got[0].AttributeExtraValue != "5" {
		t.Fatalf("quantity attribute_extra_value = %q, want 5", got[0].AttributeExtraValue)
	}
}

func TestToResolvedAttributesFallsBackQuantityExtraValueToOne(t *testing.T) {
	t.Parallel()

	valueID := 1002451
	pkg := &Package{
		ProductNameEn: "Framed Canvas Wall Art",
		ResolvedAttributes: []ResolvedAttribute{
			{
				Name:             "Quantity",
				Value:            "piece(s)",
				AttributeID:      1000411,
				AttributeValueID: &valueID,
				AttributeType:    4,
			},
		},
		RequestDraft: &RequestDraft{},
	}

	got := toResolvedAttributes(pkg)
	if len(got) != 1 {
		t.Fatalf("product attributes = %#v, want 1 attribute", got)
	}
	if got[0].AttributeExtraValue != "1" {
		t.Fatalf("quantity attribute_extra_value = %q, want fallback 1", got[0].AttributeExtraValue)
	}
}

func TestToResolvedAttributesExtractsNumericExtraValueFromResolvedValue(t *testing.T) {
	t.Parallel()

	valueID := 7001
	pkg := &Package{
		ResolvedAttributes: []ResolvedAttribute{
			{
				Name:             "Capacity",
				Value:            "500ml",
				AttributeID:      7001,
				AttributeValueID: &valueID,
				AttributeType:    4,
			},
		},
		RequestDraft: &RequestDraft{},
	}

	got := toResolvedAttributes(pkg)
	if len(got) != 1 {
		t.Fatalf("product attributes = %#v, want 1 attribute", got)
	}
	if got[0].AttributeExtraValue != "500" {
		t.Fatalf("capacity attribute_extra_value = %q, want 500", got[0].AttributeExtraValue)
	}
}

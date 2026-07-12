package shein

import "testing"

func TestReconcilePublishedSaleAttributeResolutionAddsFinalSKUAssignments(t *testing.T) {
	t.Parallel()

	sID, fiveXLID := 568, 1430561
	original := &SaleAttributeResolution{
		Status:                   "resolved",
		SecondaryAttributeID:     87,
		SecondarySourceDimension: "Size",
		SourceDimensions: []SourceVariantDimension{{
			Name:   "Size",
			Values: []string{"S", "5XL"},
		}},
		SKUValueAssignments: map[string]ResolvedSaleAttribute{
			"s": {Scope: "sku", Name: "Size", Value: "S", AttributeID: 87, AttributeValueID: &sID},
		},
	}
	pkg := &Package{DraftPayload: &RequestDraft{SKCList: []SKCRequestDraft{{
		SKUList: []SKUDraft{{
			Attributes: map[string]string{"Size": "5XL"},
			SaleAttributes: []ResolvedSaleAttribute{{
				Scope: "sku", Name: "Size", Value: "Petite GGG", AttributeID: 87, AttributeValueID: &fiveXLID,
			}},
		}},
	}}}}

	got := ReconcilePublishedSaleAttributeResolution(pkg, original)

	if got == original {
		t.Fatal("reconciliation mutated the original resolution")
	}
	assignment, ok := got.SKUValueAssignments["5xl"]
	if !ok || assignment.AttributeValueID == nil || *assignment.AttributeValueID != fiveXLID {
		t.Fatalf("5xl assignment = %+v, want attribute_value_id=%d", assignment, fiveXLID)
	}
	if _, exists := original.SKUValueAssignments["5xl"]; exists {
		t.Fatalf("original resolution was mutated: %+v", original.SKUValueAssignments)
	}
}

func TestSaleAttributeResolutionApplicableRejectsMissingCurrentValue(t *testing.T) {
	t.Parallel()

	sID := 568
	resolution := &SaleAttributeResolution{
		Status:                   "resolved",
		SecondarySourceDimension: "Size",
		SourceDimensions:         []SourceVariantDimension{{Name: "Size", Values: []string{"S", "5XL"}}},
		SKUValueAssignments: map[string]ResolvedSaleAttribute{
			"s": {AttributeID: 87, AttributeValueID: &sID},
		},
	}

	ok, reason := SaleAttributeResolutionApplicable(resolution)
	if ok {
		t.Fatal("incomplete resolution reported applicable")
	}
	if reason != "missing sku assignment for size=5xl" {
		t.Fatalf("reason = %q", reason)
	}
}

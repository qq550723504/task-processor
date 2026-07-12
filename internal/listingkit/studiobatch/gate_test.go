package studiobatch

import "testing"

func TestEvaluateGateRejectsUnapprovedDesign(t *testing.T) {
	result := EvaluateGate(GateInput{
		BatchID: "batch-1",
		Candidate: Candidate{
			Design:      Design{ID: "design-1", BatchID: "batch-1", ItemID: "item-1", ImageURL: "https://cdn.example.com/design.png"},
			Item:        Item{ID: "item-1", SelectionIDs: []string{"selection-1"}},
			SelectionID: "selection-1",
		},
		Designs: []Design{{ID: "design-1", BatchID: "batch-1", ItemID: "item-1", ImageURL: "https://cdn.example.com/design.png"}},
	})
	if result.Eligible || result.ReasonCode != "design_not_approved" {
		t.Fatalf("result = %+v", result)
	}
}

func TestEvaluateGateRejectsDesignOutsideCandidateTarget(t *testing.T) {
	result := EvaluateGate(GateInput{
		BatchID:   "batch-1",
		Candidate: Candidate{Design: Design{ID: "design-1", BatchID: "other-batch", ItemID: "item-1", Approved: true, ImageURL: "https://cdn.example.com/design.png"}, Item: Item{ID: "item-1"}},
		Designs:   []Design{{ID: "design-1", BatchID: "other-batch", ItemID: "item-1", Approved: true, ImageURL: "https://cdn.example.com/design.png"}},
	})
	if result.Eligible || result.ReasonCode != "design_target_mismatch" {
		t.Fatalf("result = %+v", result)
	}
}

func TestEvaluateGateRejectsSelectionOutsideBatch(t *testing.T) {
	result := EvaluateGate(GateInput{
		BatchID:   "batch-1",
		Candidate: Candidate{Design: Design{ID: "design-1", BatchID: "batch-1", ItemID: "item-1", Approved: true, ImageURL: "image"}, Item: Item{ID: "item-1", SelectionIDs: []string{"selection-1"}}, SelectionID: "selection-1"},
		Designs:   []Design{{ID: "design-1", BatchID: "batch-1", ItemID: "item-1", Approved: true, ImageURL: "image"}},
	})
	if result.Eligible || result.ReasonCode != "selection_not_in_batch" {
		t.Fatalf("result = %+v", result)
	}
}

func TestEvaluateGateRejectsMismatchedSelectionSnapshot(t *testing.T) {
	selection := completeGateSelection("other-selection", "https://cdn.example.com/mask.png")
	result := EvaluateGate(GateInput{
		BatchID:       "batch-1",
		Candidate:     Candidate{Design: Design{ID: "design-1", BatchID: "batch-1", ItemID: "item-1", Approved: true, ImageURL: "image"}, Item: Item{ID: "item-1", SelectionIDs: []string{"selection-1"}}, SelectionID: "selection-1", SelectionSnapshot: selection.Selection},
		Designs:       []Design{{ID: "design-1", BatchID: "batch-1", ItemID: "item-1", Approved: true, ImageURL: "image"}},
		SelectionByID: map[string]GroupedSelection{"selection-1": selection},
	})
	if result.Eligible || result.ReasonCode != "selection_not_in_batch" {
		t.Fatalf("result = %+v", result)
	}
}

func TestEvaluateGateRejectsIncompleteSelectionIdentity(t *testing.T) {
	selection := GroupedSelection{SelectionID: "selection-1", Selection: Selection{VariantID: 0, ParentProductID: 2002, PrototypeGroupID: 4004, LayerID: "layer", DesignType: "material"}}
	result := EvaluateGate(GateInput{
		BatchID:       "batch-1",
		Candidate:     Candidate{Design: Design{ID: "design-1", BatchID: "batch-1", ItemID: "item-1", Approved: true, ImageURL: "image"}, Item: Item{ID: "item-1", SelectionIDs: []string{"selection-1"}}, SelectionID: "selection-1", SelectionSnapshot: selection.Selection},
		Designs:       []Design{{ID: "design-1", BatchID: "batch-1", ItemID: "item-1", Approved: true, ImageURL: "image"}},
		SelectionByID: map[string]GroupedSelection{"selection-1": selection},
	})
	if result.Eligible || result.ReasonCode != "selection_identity_incomplete" {
		t.Fatalf("result = %+v", result)
	}
}

func TestEvaluateGateRejectsIncompatibleVariantSurface(t *testing.T) {
	selection := GroupedSelection{SelectionID: "selection-1", Selection: Selection{VariantID: 6006, ParentProductID: 2002, PrototypeGroupID: 4004, LayerID: "layer", DesignType: "material", Variants: []VariantSurface{{VariantID: 6006, PrototypeGroupID: 9999, LayerID: "layer"}}}}
	result := EvaluateGate(GateInput{
		BatchID:       "batch-1",
		Candidate:     Candidate{Design: Design{ID: "design-1", BatchID: "batch-1", ItemID: "item-1", Approved: true, ImageURL: "image"}, Item: Item{ID: "item-1", SelectionIDs: []string{"selection-1"}}, SelectionID: "selection-1", SelectionSnapshot: selection.Selection},
		Designs:       []Design{{ID: "design-1", BatchID: "batch-1", ItemID: "item-1", Approved: true, ImageURL: "image"}},
		SelectionByID: map[string]GroupedSelection{"selection-1": selection},
	})
	if result.Eligible || result.ReasonCode != "selection_variant_incompatible" {
		t.Fatalf("result = %+v", result)
	}
}

func TestEvaluateGateRejectsUnknownSelectedVariant(t *testing.T) {
	selection := GroupedSelection{SelectionID: "selection-1", Selection: Selection{VariantID: 6006, ParentProductID: 2002, PrototypeGroupID: 4004, LayerID: "layer", DesignType: "material", PrintableWidth: 1200, PrintableHeight: 1200, TemplateImageURL: "template", SelectedVariantIDs: []int64{7007}, Variants: []VariantSurface{{VariantID: 6006, PrototypeGroupID: 4004, LayerID: "layer"}}}}
	result := EvaluateGate(GateInput{
		BatchID:       "batch-1",
		Candidate:     Candidate{Design: Design{ID: "design-1", BatchID: "batch-1", ItemID: "item-1", Approved: true, ImageURL: "image"}, Item: Item{ID: "item-1", SelectionIDs: []string{"selection-1"}}, SelectionID: "selection-1", SelectionSnapshot: selection.Selection},
		Designs:       []Design{{ID: "design-1", BatchID: "batch-1", ItemID: "item-1", Approved: true, ImageURL: "image"}},
		SelectionByID: map[string]GroupedSelection{"selection-1": selection},
	})
	if result.Eligible || result.ReasonCode != "selection_variant_incompatible" {
		t.Fatalf("result = %+v", result)
	}
}

func TestEvaluateGateRejectsGroupedCompatibilityMismatch(t *testing.T) {
	first := completeGateSelection("selection-1", "https://cdn.example.com/mask-a.png")
	second := completeGateSelection("selection-2", "https://cdn.example.com/mask-b.png")
	result := EvaluateGate(GateInput{
		BatchID: "batch-1", BatchGroupMode: "shared_by_size",
		Candidate:     Candidate{Design: Design{ID: "design-1", BatchID: "batch-1", ItemID: "item-1", Approved: true, ImageURL: "image"}, Item: Item{ID: "item-1", GroupMode: "shared_by_size", SelectionIDs: []string{"selection-1", "selection-2"}}, SelectionID: "selection-1", SelectionSnapshot: first.Selection},
		Designs:       []Design{{ID: "design-1", BatchID: "batch-1", ItemID: "item-1", Approved: true, ImageURL: "image"}},
		SelectionByID: map[string]GroupedSelection{"selection-1": first, "selection-2": second}, ItemSelections: []GroupedSelection{first, second},
	})
	if result.Eligible || result.ReasonCode != "compatibility_mismatch" {
		t.Fatalf("result = %+v", result)
	}
}

func completeGateSelection(id, mask string) GroupedSelection {
	return GroupedSelection{SelectionID: id, Selection: Selection{VariantID: 6006, ParentProductID: 2002, PrototypeGroupID: 4004, LayerID: "layer", DesignType: "material", PrintableWidth: 1200, PrintableHeight: 1200, TemplateImageURL: "https://cdn.example.com/template.png", MaskImageURL: mask}}
}

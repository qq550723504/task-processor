package studiobatch

import "testing"

func TestEvaluateUsesExplicitItemSelectionOwnership(t *testing.T) {
	result := Evaluate(EvaluationInput{
		BatchID: "batch-1",
		Item:    Item{ID: "item-1", GroupMode: "per_product"},
		Design:  Design{ID: "design-1"},
		ResolvedSelections: []GroupedSelection{{
			SelectionID: "selected-1",
			Selection:   Selection{VariantID: 10, DesignType: "material"},
		}},
		ExplicitSelectionOwnership: true,
	})
	if len(result.Candidates) != 1 || result.Candidates[0].SelectionID != "selected-1" {
		t.Fatalf("candidates = %+v", result.Candidates)
	}
}

func TestEvaluateRejectsPerProductMultipleSelections(t *testing.T) {
	result := Evaluate(EvaluationInput{
		BatchID: "batch-1",
		Item:    Item{ID: "item-1", GroupMode: "per_product"},
		Design:  Design{ID: "design-1"},
		ResolvedSelections: []GroupedSelection{
			{SelectionID: "first", Selection: Selection{VariantID: 10}},
			{SelectionID: "second", Selection: Selection{VariantID: 20}},
		},
		ExplicitSelectionOwnership: true,
	})
	if len(result.Candidates) != 0 || len(result.Rejections) != 1 ||
		result.Rejections[0].ReasonCode != "selection_cardinality_mismatch" {
		t.Fatalf("result = %+v", result)
	}
}

func TestEvaluateCandidateKeyChangesWhenProductSizeChanges(t *testing.T) {
	input := EvaluationInput{
		TenantID: "tenant-1",
		BatchID:  "batch-1",
		Item:     Item{ID: "item-1"},
		Design:   Design{ID: "design-1"},
		ResolvedSelections: []GroupedSelection{{
			SelectionID: "selection-1",
			StoreID:     1043,
			Selection: Selection{
				VariantID:   10,
				ProductSize: `[[{"content":"size"},{"content":"length"}]]`,
			},
		}},
		ExplicitSelectionOwnership: true,
	}
	first := Evaluate(input)
	input.ResolvedSelections[0].Selection.ProductSize = `[[{"content":"size"},{"content":"width"}]]`
	second := Evaluate(input)
	if len(first.Candidates) != 1 || len(second.Candidates) != 1 {
		t.Fatalf("candidates = first %+v second %+v", first.Candidates, second.Candidates)
	}
	if first.Candidates[0].CandidateKey == second.Candidates[0].CandidateKey {
		t.Fatalf("candidate keys unexpectedly match: %q", first.Candidates[0].CandidateKey)
	}
}

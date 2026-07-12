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

func TestEvaluateUsesLegacyCompatibilityFingerprintShape(t *testing.T) {
	result := Evaluate(EvaluationInput{
		BatchID: "batch-1",
		Item:    Item{ID: "item-1"},
		Design:  Design{ID: "design-1"},
		ResolvedSelections: []GroupedSelection{{
			SelectionID: "selection-1",
			Selection: Selection{
				ParentProductID:  2002,
				PrototypeGroupID: 4004,
				LayerID:          "layer-front",
				DesignType:       "material",
				PrintableWidth:   1200,
				PrintableHeight:  1200,
				TemplateImageURL: "https://cdn.example.com/template-a.png",
				MaskImageURL:     "https://cdn.example.com/mask-a.png",
				ProductSize:      "size-table",
				PackagingSpec:    "packaging",
			},
		}},
		ExplicitSelectionOwnership: true,
	})
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %+v", result.Candidates)
	}
	if got, want := result.Candidates[0].CompatibilityFingerprint, "1e3ae60e6b772de3f27236c69ef0f20416f0a883"; got != want {
		t.Fatalf("fingerprint = %q, want %q", got, want)
	}
	if got, want := result.Candidates[0].StyleID, "2FCCEB8A80"; got != want {
		t.Fatalf("style ID = %q, want %q", got, want)
	}
}

func TestEvaluateDerivesSelectionIDFromSelectionSnapshot(t *testing.T) {
	result := Evaluate(EvaluationInput{
		BatchID: "batch-1",
		Item:    Item{ID: "item-1"},
		Design:  Design{ID: "design-1"},
		ResolvedSelections: []GroupedSelection{{
			Selection: Selection{
				ParentProductID:    2002,
				PrototypeGroupID:   4004,
				VariantID:          6006,
				LayerID:            "layer-front",
				SelectedVariantIDs: []int64{6006, 6007},
			},
		}},
		ExplicitSelectionOwnership: true,
	})
	if len(result.Candidates) != 1 {
		t.Fatalf("candidates = %+v", result.Candidates)
	}
	if got, want := result.Candidates[0].SelectionID, "2002:4004:6006:layer-front:6006,6007"; got != want {
		t.Fatalf("selection ID = %q, want %q", got, want)
	}
}

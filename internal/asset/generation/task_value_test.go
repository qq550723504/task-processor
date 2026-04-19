package generation

import "testing"

func TestTaskValueRoundTrip(t *testing.T) {
	t.Parallel()

	original := &Task{
		TaskID:          "listing-task-1",
		ID:              "shein:shein-main-model",
		Platform:        "shein",
		RecipeID:        "shein-main-model",
		AssetKind:       "model_image",
		Slot:            "main",
		Purpose:         "main",
		TemplateLabel:   "SHEIN Editorial Main",
		RenderProfile:   "shein_model_editorial",
		Status:          "completed",
		ExecutionStatus: "completed",
		ExecutionMode:   "deferred_stub",
		CanExecute:      true,
		SatisfiedBy:     "generated_asset",
		FallbackFrom:    "",
		Lineage:         []string{"shein", "shein-main-model"},
		SourceAssetIDs:  []string{"source-1", "source-2"},
	}

	value, err := original.Value()
	if err != nil {
		t.Fatalf("Value() error = %v", err)
	}

	var decoded Task
	if err := decoded.Scan(value); err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if decoded.TaskID != original.TaskID || decoded.ID != original.ID {
		t.Fatalf("decoded = %+v, want task identity from %+v", decoded, *original)
	}
	if len(decoded.SourceAssetIDs) != len(original.SourceAssetIDs) || decoded.SourceAssetIDs[0] != original.SourceAssetIDs[0] {
		t.Fatalf("decoded = %+v, want source asset ids from %+v", decoded, *original)
	}
}

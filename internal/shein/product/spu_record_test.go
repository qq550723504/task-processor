package product

import (
	"testing"

	"task-processor/internal/model"
	shein "task-processor/internal/shein"
)

func TestHasSpuRecordHandlerExtractAsinsFromContextPrefersLoadedVariants(t *testing.T) {
	handler := NewHasSpuRecordHandler()
	ctx := shein.NewTaskContext(nil, &model.Task{ProductID: "MAIN-ASIN"})
	ctx.AmazonProduct = &model.Product{
		Asin: "MAIN-ASIN",
		Variations: []model.Variation{
			{Asin: "A1"},
			{Asin: "A2"},
		},
	}
	ctx.SetVariants([]model.Product{
		{Asin: "A2"},
		{Asin: "A3"},
		{Asin: "A4"},
	})

	got := handler.extractAsinsFromContext(ctx)
	want := []string{"MAIN-ASIN", "A2", "A3", "A4"}

	if len(got) != len(want) {
		t.Fatalf("extractAsinsFromContext() len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("extractAsinsFromContext()[%d] = %q, want %q (full=%v)", i, got[i], want[i], got)
		}
	}
}

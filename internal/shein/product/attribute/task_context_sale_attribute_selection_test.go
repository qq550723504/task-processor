package attribute_test

import (
	"context"
	"testing"

	sheinctx "task-processor/internal/shein/context"
)

func TestTaskContextStoresSaleAttributeSelectionState(t *testing.T) {
	taskCtx := sheinctx.NewTaskContext(context.Background(), nil)
	expected := &sheinctx.SaleAttributeSelectionState{
		Source:                   "resolver",
		PrimaryAttributeID:       501,
		SecondaryAttributeID:     502,
		PrimarySourceDimension:   "Color",
		SecondarySourceDimension: "Size",
	}

	if taskCtx.SaleAttributeSelection != nil {
		t.Fatalf("SaleAttributeSelection = %+v, want nil before storage", taskCtx.SaleAttributeSelection)
	}

	taskCtx.SetSaleAttributeSelection(expected)

	if taskCtx.SaleAttributeSelection != expected {
		t.Fatalf("SaleAttributeSelection = %+v, want %+v", taskCtx.SaleAttributeSelection, expected)
	}
	if taskCtx.SaleAttributeSelection.Source != "resolver" {
		t.Fatalf("SaleAttributeSelection.Source = %q, want %q", taskCtx.SaleAttributeSelection.Source, "resolver")
	}
	if taskCtx.SaleAttributeSelection.PrimaryAttributeID != 501 {
		t.Fatalf("SaleAttributeSelection.PrimaryAttributeID = %d, want %d", taskCtx.SaleAttributeSelection.PrimaryAttributeID, 501)
	}
	if taskCtx.SaleAttributeSelection.SecondaryAttributeID != 502 {
		t.Fatalf("SaleAttributeSelection.SecondaryAttributeID = %d, want %d", taskCtx.SaleAttributeSelection.SecondaryAttributeID, 502)
	}
	if taskCtx.SaleAttributeSelection.PrimarySourceDimension != "Color" {
		t.Fatalf("SaleAttributeSelection.PrimarySourceDimension = %q, want %q", taskCtx.SaleAttributeSelection.PrimarySourceDimension, "Color")
	}
	if taskCtx.SaleAttributeSelection.SecondarySourceDimension != "Size" {
		t.Fatalf("SaleAttributeSelection.SecondarySourceDimension = %q, want %q", taskCtx.SaleAttributeSelection.SecondarySourceDimension, "Size")
	}
}

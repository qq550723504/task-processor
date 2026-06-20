package validation

import (
	"testing"

	"task-processor/internal/listingruntime"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	sheinctx "task-processor/internal/shein/context"
)

func TestApplyFilterRuleHandlerDefersInventoryCheckForVariantProducts(t *testing.T) {
	stockMin := 5
	handler := NewApplyFilterRuleHandler()
	ctx := &sheinctx.TaskContext{
		RuntimeState: sheinctx.RuntimeState{
			StoreInfo: &listingruntime.StoreInfo{PriceType: "special"},
		},
		ProductState: sheinctx.ProductState{
			FilterRule: &listingruntime.FilterRule{StockMin: &stockMin},
			AmazonProduct: &model.Product{
				Asin:                 "B0TESTVARIANT",
				IsAvailable:          true,
				MaxQuantityAvailable: 1,
				Variations: []model.Variation{
					{Name: "Size", Asin: "B0TESTVARIANT-1"},
					{Name: "Size", Asin: "B0TESTVARIANT-2"},
				},
			},
		},
	}

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("expected variant product inventory check to be deferred, got error: %v", err)
	}
}

func TestApplyFilterRuleHandlerStillFiltersSingleSKUByInventory(t *testing.T) {
	stockMin := 5
	handler := NewApplyFilterRuleHandler()
	ctx := &sheinctx.TaskContext{
		RuntimeState: sheinctx.RuntimeState{
			StoreInfo: &listingruntime.StoreInfo{PriceType: "special"},
		},
		ProductState: sheinctx.ProductState{
			FilterRule: &listingruntime.FilterRule{StockMin: &stockMin},
			AmazonProduct: &model.Product{
				Asin:                 "B0TESTSINGLE",
				IsAvailable:          true,
				MaxQuantityAvailable: 1,
			},
		},
	}

	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("expected single-SKU product to be filtered by inventory")
	}
	if !shein.IsFilteredError(err) {
		t.Fatalf("expected filtered error, got %T", err)
	}
}

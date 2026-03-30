package publish

import (
	"testing"

	managementapi "task-processor/internal/infra/clients/management/api"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestSavePublishResultCalculateIncrementUsesStoreLimitType(t *testing.T) {
	handler := NewSavePublishResultHandler()
	limit := 100
	input := &PublishResultInput{
		StoreInfo: &managementapi.StoreRespDTO{
			DailyLimit:     &limit,
			DailyLimitType: "SKU",
		},
		SheinResponse: &sheinproduct.SheinResponse{
			Info: sheinproduct.ResponseInfo{
				SKCList: []sheinproduct.ResponseSKC{
					{
						SKUList: []sheinproduct.ResponseSKU{
							{SKUCode: "sku-1"},
							{SKUCode: "sku-2"},
						},
					},
					{
						SKUList: []sheinproduct.ResponseSKU{
							{SKUCode: "sku-3"},
						},
					},
				},
			},
		},
	}

	got := handler.calculateIncrement(input)
	if got != 3 {
		t.Fatalf("expected SKU increment 3, got %d", got)
	}
}


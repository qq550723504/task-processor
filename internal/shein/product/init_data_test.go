package product

import (
	"testing"

	"task-processor/internal/shein/authorizedbrand"
	sheinctx "task-processor/internal/shein/context"
)

func TestInitProductDataHandlerSetsAuthorizedBrandCode(t *testing.T) {
	handler := NewInitProductDataHandler()
	ctx := &sheinctx.TaskContext{
		RuntimeState: sheinctx.RuntimeState{
			AuthorizedBrand: &authorizedbrand.Resolved{
				Enabled: true,
				Code:    "2fd1n",
				Name:    "Logitech",
			},
		},
	}

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}
	if ctx.ProductData == nil {
		t.Fatal("ProductData = nil, want initialized product")
	}
	if ctx.ProductData.BrandCode == nil || *ctx.ProductData.BrandCode != "2fd1n" {
		t.Fatalf("ProductData.BrandCode = %#v, want 2fd1n", ctx.ProductData.BrandCode)
	}
}

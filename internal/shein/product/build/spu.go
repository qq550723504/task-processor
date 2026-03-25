package build

import (
	"fmt"

	"github.com/google/uuid"

	shein "task-processor/internal/shein"
	productapi "task-processor/internal/shein/api/product"
	sheinproduct "task-processor/internal/shein/product"
)

type BuildSpuHandler struct{}

func NewBuildSpuHandler() *BuildSpuHandler {
	return &BuildSpuHandler{}
}

func (h *BuildSpuHandler) Name() string {
	return "build_spu"
}

func (h *BuildSpuHandler) Handle(ctx *shein.TaskContext) error {
	if ctx.ProductData == nil {
		return fmt.Errorf("product data is not initialized")
	}
	buildSpuData(ctx)
	return nil
}

func buildSpuData(ctx *shein.TaskContext) {
	supplierCode := sheinproduct.GetSkuByAsin(ctx, ctx.Task.ProductID)
	ctx.UpdateProductData(func(productData *productapi.Product) {
		productData.SupplierCode = supplierCode
		productData.PointKey = uuid.New().String()
	})
}

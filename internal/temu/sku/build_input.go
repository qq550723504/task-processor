package sku

import (
	"fmt"

	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"
)

type SKUProcessInput struct {
	Product  *models.Product
	Runtime  *temucontext.SKUBuildRuntime
	SKCIndex int
	SKUIndex int
}

func buildSKUProcessInput(temuCtx *temucontext.TemuTaskContext, skcIndex, skuIndex int) (*SKUProcessInput, error) {
	if temuCtx == nil {
		return nil, fmt.Errorf("temu context is nil")
	}
	if temuCtx.TemuProduct == nil {
		return nil, fmt.Errorf("temu product is not initialized")
	}

	product := temuCtx.TemuProduct
	if skcIndex >= len(product.SkcList) || skuIndex >= len(product.SkcList[skcIndex].SkuList) {
		return nil, fmt.Errorf("sku index out of range")
	}

	runtime, err := temucontext.BuildSKUBuildRuntime(temuCtx)
	if err != nil {
		runtime = nil
	}

	return &SKUProcessInput{
		Product:  product,
		Runtime:  runtime,
		SKCIndex: skcIndex,
		SKUIndex: skuIndex,
	}, nil
}

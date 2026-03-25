package sku

import (
	"fmt"

	"task-processor/internal/model"
	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"
)

type SKUVariantBuildInput struct {
	Runtime *temucontext.SKUBuildRuntime
	Variant *model.Product
	AISKU   temucontext.AIGeneratedSku
}

type SKUProcessInput struct {
	Product  *models.Product
	Runtime  *temucontext.SKUBuildRuntime
	SKCIndex int
	SKUIndex int
}

func buildSKUVariantBuildInputWithRuntime(
	runtime *temucontext.SKUBuildRuntime,
	variant *model.Product,
	aiSKU temucontext.AIGeneratedSku,
) (*SKUVariantBuildInput, error) {
	if variant == nil {
		return nil, fmt.Errorf("variant is nil")
	}

	return &SKUVariantBuildInput{
		Runtime: runtime,
		Variant: variant,
		AISKU:   aiSKU,
	}, nil
}

func buildSKUVariantBuildInput(
	temuCtx *temucontext.TemuTaskContext,
	variant *model.Product,
	aiSKU temucontext.AIGeneratedSku,
) (*SKUVariantBuildInput, error) {
	if temuCtx == nil {
		return nil, fmt.Errorf("temu context is nil")
	}
	if variant == nil {
		return nil, fmt.Errorf("variant is nil")
	}

	runtime, err := temucontext.BuildSKUBuildRuntime(temuCtx)
	if err != nil {
		runtime = nil
	}

	return buildSKUVariantBuildInputWithRuntime(runtime, variant, aiSKU)
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

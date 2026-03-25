package sku

import (
	"fmt"

	"task-processor/internal/model"
	temucontext "task-processor/internal/temu/context"
)

type AIHandlerInput struct {
	Variants []*model.Product
	Source   string
}

func buildAIHandlerInput(temuCtx *temucontext.TemuTaskContext) (*AIHandlerInput, error) {
	if temuCtx == nil {
		return nil, fmt.Errorf("temu context is nil")
	}

	if variants := temuCtx.GetVariants(); len(variants) > 0 {
		return &AIHandlerInput{
			Variants: variants,
			Source:   "variants",
		}, nil
	}

	if amazonProduct := temuCtx.GetAmazonProduct(); amazonProduct != nil {
		return &AIHandlerInput{
			Variants: []*model.Product{amazonProduct},
			Source:   "amazon_product",
		}, nil
	}

	return &AIHandlerInput{
		Variants: []*model.Product{},
		Source:   "empty",
	}, nil
}

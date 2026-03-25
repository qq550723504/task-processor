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

type AIBatchInput struct {
	Variants  []*model.Product
	BatchSize int
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

func buildAIBatchInput(variants []*model.Product, batchSize int) (*AIBatchInput, error) {
	if batchSize <= 0 {
		return nil, fmt.Errorf("batch size must be greater than 0")
	}

	return &AIBatchInput{
		Variants:  variants,
		BatchSize: batchSize,
	}, nil
}

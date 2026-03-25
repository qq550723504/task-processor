package sale

import (
	"context"

	"task-processor/internal/model"
	productapi "task-processor/internal/shein/api/product"
	sheinctx "task-processor/internal/shein/context"
)

type SaleAttributeInput struct {
	Context       context.Context
	Task          *model.Task
	AmazonProduct *model.Product
	Variants      []model.Product
	ProductData   *productapi.Product
}

func newSaleAttributeInput(ctx *sheinctx.TaskContext) *SaleAttributeInput {
	input := &SaleAttributeInput{
		Context:       ctx.Context,
		Task:          ctx.Task,
		AmazonProduct: ctx.AmazonProduct,
		ProductData:   ctx.ProductData,
	}
	if ctx.Variants != nil {
		input.Variants = append([]model.Product{}, (*ctx.Variants)...)
	}
	return input
}

func (in *SaleAttributeInput) HasVariants() bool {
	return len(in.Variants) > 0
}

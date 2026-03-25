package sale

import (
	"task-processor/internal/model"
	sheinctx "task-processor/internal/shein/context"
)

type SmartFilterInput struct {
	AmazonProduct *model.Product
	Variants      []model.Product
}

func NewSmartFilterInput(ctx *sheinctx.TaskContext) *SmartFilterInput {
	input := &SmartFilterInput{
		AmazonProduct: ctx.AmazonProduct,
	}
	if ctx.Variants != nil {
		input.Variants = append([]model.Product(nil), (*ctx.Variants)...)
	}
	return input
}

package sku

import (
	"fmt"

	"task-processor/internal/model"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheinctx "task-processor/internal/shein/context"
)

type RuntimeInput struct {
	AmazonProduct      *model.Product
	Variants           []model.Product
	AttributeTemplates *sheinattribute.AttributeTemplateInfo
}

func newRuntimeInput(ctx *sheinctx.TaskContext) *RuntimeInput {
	input := &RuntimeInput{
		AmazonProduct:      ctx.AmazonProduct,
		AttributeTemplates: ctx.AttributeTemplates,
	}
	if ctx.Variants != nil {
		input.Variants = append([]model.Product(nil), (*ctx.Variants)...)
	}
	return input
}

func (in *RuntimeInput) Validate() error {
	if in == nil {
		return fmt.Errorf("SKU runtime input is not initialized")
	}
	if in.AmazonProduct == nil {
		return fmt.Errorf("amazon product is not initialized")
	}
	if in.AttributeTemplates == nil {
		return fmt.Errorf("attribute templates are not initialized")
	}
	return nil
}

func (in *RuntimeInput) FindProductInfoByASIN(asin string) *model.Product {
	for i := range in.Variants {
		if in.Variants[i].Asin == asin {
			return &in.Variants[i]
		}
	}
	return in.AmazonProduct
}

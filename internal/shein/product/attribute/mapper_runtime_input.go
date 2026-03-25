package attribute

import (
	"fmt"

	sheinapi "task-processor/internal/shein/api/attribute"
	sheinctx "task-processor/internal/shein/context"
)

type MapperRuntimeInput struct {
	CategoryID         int
	ProductTitle       string
	AttributeTemplates *sheinapi.AttributeTemplateInfo
	AttributeAPI       *sheinapi.Client
}

func newMapperRuntimeInput(ctx *sheinctx.TaskContext) *MapperRuntimeInput {
	input := &MapperRuntimeInput{
		AttributeTemplates: ctx.AttributeTemplates,
		AttributeAPI:       ctx.AttributeAPI,
	}
	if ctx.ProductData != nil {
		input.CategoryID = ctx.ProductData.CategoryID
	}
	if ctx.AmazonProduct != nil {
		input.ProductTitle = ctx.AmazonProduct.Title
	}
	return input
}

func (in *MapperRuntimeInput) Validate() error {
	if in == nil {
		return fmt.Errorf("attribute mapper runtime input is not initialized")
	}
	if in.AttributeTemplates == nil {
		return fmt.Errorf("attribute templates are not initialized")
	}
	if in.AttributeAPI == nil {
		return fmt.Errorf("attribute API is not initialized")
	}
	if in.CategoryID == 0 {
		return fmt.Errorf("category ID is not initialized")
	}
	if in.ProductTitle == "" {
		return fmt.Errorf("product title is not initialized")
	}
	return nil
}

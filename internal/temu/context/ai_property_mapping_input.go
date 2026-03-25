package context

import (
	"fmt"

	temutemplate "task-processor/internal/temu/api/template"
)

type PropertyMappingInput struct {
	AmazonProduct AmazonProductData
	TemplateInfo  *temutemplate.TemplateInfo
}

func BuildPropertyMappingInput(temuCtx *TemuTaskContext) (*PropertyMappingInput, error) {
	if temuCtx == nil {
		return nil, fmt.Errorf("temu context is nil")
	}
	if temuCtx.TemplateInfo == nil {
		return nil, fmt.Errorf("template info is nil")
	}

	input := &PropertyMappingInput{
		TemplateInfo: temuCtx.TemplateInfo,
	}
	if temuCtx.GetAmazonProduct() != nil {
		input.AmazonProduct = ConvertAmazonProductData(temuCtx.GetAmazonProduct())
	}
	return input, nil
}

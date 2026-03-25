package build

import (
	"fmt"

	shein "task-processor/internal/shein"
	"task-processor/internal/shein/api/attribute"
	productapi "task-processor/internal/shein/api/product"
	salesmart "task-processor/internal/shein/product/attribute/sale"
)

type BuildAttributeInput struct {
	AttributeTemplates *attribute.AttributeTemplateInfo
	SmartFilterInput   *salesmart.SmartFilterInput
}

func buildAttributeInput(ctx *shein.TaskContext) (*BuildAttributeInput, error) {
	if ctx == nil {
		return nil, fmt.Errorf("task context is nil")
	}
	if ctx.AttributeTemplates == nil || len(ctx.AttributeTemplates.Data) == 0 {
		return nil, fmt.Errorf("attribute templates are empty")
	}

	return &BuildAttributeInput{
		AttributeTemplates: ctx.AttributeTemplates,
		SmartFilterInput:   salesmart.NewSmartFilterInput(ctx),
	}, nil
}

type BuildSpuInput struct {
	TaskID      string
	AsinSkuMap  map[string]string
	ProductData *productapi.Product
}

func buildSpuInput(ctx *shein.TaskContext) (*BuildSpuInput, error) {
	if ctx == nil {
		return nil, fmt.Errorf("task context is nil")
	}
	if ctx.ProductData == nil {
		return nil, fmt.Errorf("product data is not initialized")
	}
	if ctx.Task == nil {
		return nil, fmt.Errorf("task is nil")
	}

	return &BuildSpuInput{
		TaskID:      ctx.Task.ProductID,
		AsinSkuMap:  ctx.AsinSkuMap,
		ProductData: ctx.ProductData,
	}, nil
}

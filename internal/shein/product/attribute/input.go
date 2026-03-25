package attribute

import (
	"context"
	"fmt"

	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/shein/aicache"
	apiattribute "task-processor/internal/shein/api/attribute"
	productapi "task-processor/internal/shein/api/product"
	sheinctx "task-processor/internal/shein/context"
)

type AttributeSelectionInput struct {
	Context            context.Context
	CacheKey           string
	AICache            *aicache.Cache
	ProductData        *productapi.Product
	BuildAttributeData *BuildAttributeInfo
	AttributeTemplates *apiattribute.AttributeTemplateInfo
	AmazonProductTitle string
	OpenAIClient       openaiClient.ChatCompleter
}

func buildAttributeSelectionInput(ctx *sheinctx.TaskContext, client openaiClient.ChatCompleter) (*AttributeSelectionInput, error) {
	if ctx == nil {
		return nil, fmt.Errorf("task context is nil")
	}
	if ctx.ProductData == nil {
		return nil, fmt.Errorf("product data is not initialized")
	}
	if ctx.BuildAttributeData == nil || len(ctx.BuildAttributeData.AttributeData) == 0 {
		return nil, fmt.Errorf("build attribute data is empty")
	}
	if ctx.AttributeTemplates == nil {
		return nil, fmt.Errorf("attribute templates is nil")
	}
	if ctx.AmazonProduct == nil {
		return nil, fmt.Errorf("amazon product is nil")
	}

	return &AttributeSelectionInput{
		Context:            ctx.Context,
		CacheKey:           fmt.Sprintf("%s:%d", ctx.AmazonProduct.Asin, ctx.ProductData.CategoryID),
		AICache:            ctx.AICache,
		ProductData:        ctx.ProductData,
		BuildAttributeData: ctx.BuildAttributeData,
		AttributeTemplates: ctx.AttributeTemplates,
		AmazonProductTitle: ctx.AmazonProduct.Title,
		OpenAIClient:       client,
	}, nil
}

type FillAttributeInput struct {
	ProductData        *productapi.Product
	GenerateAttribute  *AttributeData
	AttributeTemplates *apiattribute.AttributeTemplateInfo
}

func buildFillAttributeInput(ctx *sheinctx.TaskContext) (*FillAttributeInput, error) {
	if ctx == nil {
		return nil, fmt.Errorf("task context is nil")
	}
	if ctx.GenerateAttribute == nil {
		return nil, fmt.Errorf("generated attribute data is nil")
	}
	if ctx.ProductData == nil {
		return nil, fmt.Errorf("product data is nil")
	}

	return &FillAttributeInput{
		ProductData:        ctx.ProductData,
		GenerateAttribute:  ctx.GenerateAttribute,
		AttributeTemplates: ctx.AttributeTemplates,
	}, nil
}

type AttributeTemplateInput struct {
	CategoryID   int
	AttributeAPI apiattribute.AttributeAPI
}

func buildAttributeTemplateInput(ctx *sheinctx.TaskContext) (*AttributeTemplateInput, error) {
	if ctx == nil {
		return nil, fmt.Errorf("task context is nil")
	}
	if ctx.ProductData == nil {
		return nil, fmt.Errorf("product data is nil")
	}
	if ctx.ProductData.CategoryID == 0 {
		return nil, fmt.Errorf("category id is not set")
	}
	if ctx.AttributeAPI == nil {
		return nil, fmt.Errorf("attribute api is nil")
	}

	return &AttributeTemplateInput{
		CategoryID:   ctx.ProductData.CategoryID,
		AttributeAPI: ctx.AttributeAPI,
	}, nil
}

type ValidateRepairInput struct {
	SaleSpecResult     *ResultSaleAttribute
	AttributeTemplates *apiattribute.AttributeTemplateInfo
}

func buildValidateRepairInput(ctx *sheinctx.TaskContext) (*ValidateRepairInput, error) {
	if ctx == nil {
		return nil, fmt.Errorf("task context is nil")
	}
	if ctx.ProductData == nil {
		return nil, fmt.Errorf("product data is not initialized")
	}
	if ctx.SaleSpecResult == nil {
		return nil, fmt.Errorf("sale spec result is nil")
	}

	return &ValidateRepairInput{
		SaleSpecResult:     ctx.SaleSpecResult,
		AttributeTemplates: ctx.AttributeTemplates,
	}, nil
}

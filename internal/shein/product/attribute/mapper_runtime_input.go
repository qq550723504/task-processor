package attribute

import (
	"fmt"
	"strings"

	"task-processor/internal/model"
	"task-processor/internal/shein/aicache"
	sheinapi "task-processor/internal/shein/api/attribute"
	sheinctx "task-processor/internal/shein/context"
)

type MapperRuntimeInput struct {
	CategoryID            int
	ProductTitle          string
	AmazonProduct         *model.Product
	AttributeTemplates    *sheinapi.AttributeTemplateInfo
	AttributeAPI          *sheinapi.Client
	FallbackValueResolver platformValueFallbackResolver
	FallbackCache         *aicache.Cache
	FallbackMinConfidence float64
}

func newMapperRuntimeInput(ctx *sheinctx.TaskContext) *MapperRuntimeInput {
	input := &MapperRuntimeInput{
		AttributeTemplates: ctx.AttributeTemplates,
		AttributeAPI:       ctx.AttributeAPI,
		FallbackCache:      ctx.AICache,
	}
	if ctx.ProductData != nil {
		input.CategoryID = ctx.ProductData.CategoryID
	}
	if ctx.AmazonProduct != nil {
		input.ProductTitle = ctx.AmazonProduct.Title
		input.AmazonProduct = ctx.AmazonProduct
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

func (in *MapperRuntimeInput) buildFallbackRequest(attrID int, domain platformValueDomain, rawValue string, platformValues map[string]int) *PlatformValueFallbackRequest {
	if in == nil {
		return nil
	}
	return &PlatformValueFallbackRequest{
		AttrID:         attrID,
		Domain:         domain,
		RawValue:       rawValue,
		ProductTitle:   in.ProductTitle,
		PlatformValues: stablePlatformValues(platformValues, 30),
		SizeChart:      formatSizeChartForPrompt(in.AmazonProduct),
	}
}

func (in *MapperRuntimeInput) buildFallbackCacheKey(domain platformValueDomain, attrID int, rawValue string, platformValues map[string]int) string {
	return aicache.HashKey(
		fmt.Sprintf("%d", attrID),
		string(domain),
		strings.TrimSpace(rawValue),
		strings.Join(stablePlatformValues(platformValues, 30), "|"),
		formatSizeChartForPrompt(in.AmazonProduct),
	)
}

func formatSizeChartForPrompt(product *model.Product) string {
	if product == nil || product.SizeChart == nil {
		return ""
	}
	var lines []string
	if len(product.SizeChart.Headers) > 0 {
		lines = append(lines, strings.Join(product.SizeChart.Headers, " | "))
	}
	for _, row := range product.SizeChart.Rows {
		lines = append(lines, strings.Join(row, " | "))
	}
	return strings.Join(lines, "\n")
}

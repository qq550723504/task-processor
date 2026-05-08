package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"task-processor/internal/catalog/canonical"
	productenrich "task-processor/internal/productenrich"

	"github.com/sirupsen/logrus"
)

func (g *jsonGenerator) GenerateJSON(ctx context.Context, analysis *productenrich.ProductAnalysis, variantGen productenrich.VariantGenerator, skipVariants bool) (*productenrich.ProductJSON, error) {
	if analysis == nil {
		return nil, fmt.Errorf("analysis cannot be nil")
	}

	g.logger.Info("generating product JSON")

	productJSON, err := g.generateWithLLM(ctx, analysis)
	if err != nil {
		g.logger.WithError(err).Warn("LLM generation failed, falling back to analysis data")
		productJSON = g.fallbackFromAnalysis(analysis)
	}

	if variantGen != nil {
		if specs, err := variantGen.GenerateSpecs(ctx, analysis); err != nil {
			logrus.WithError(err).Warn("failed to generate specs")
		} else {
			productJSON.Specifications = specs
		}

		if !skipVariants {
			if variants, err := variantGen.GenerateVariants(ctx, analysis); err != nil {
				logrus.WithError(err).Warn("failed to generate variants")
			} else {
				productJSON.Variants = variants
			}
		}
	}
	if len(productJSON.VariantDimensions) == 0 && analysis.ScrapedData != nil && len(analysis.ScrapedData.VariantDimensions) > 0 {
		productJSON.VariantDimensions = append([]canonical.ScrapedVariantDimension(nil), analysis.ScrapedData.VariantDimensions...)
	}
	applySourceBackedAttributes(productJSON, analysis)

	g.logger.Info("product JSON generated successfully")
	return productJSON, nil
}

func (g *jsonGenerator) generateWithLLM(ctx context.Context, analysis *productenrich.ProductAnalysis) (*productenrich.ProductJSON, error) {
	client := g.llmManager.GetDefaultClient()

	prompt := g.buildPrompt(analysis)
	response, err := client.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var result productenrich.ProductJSON
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}
	return &result, nil
}

func (g *jsonGenerator) buildPrompt(analysis *productenrich.ProductAnalysis) string {
	fallback := `You are an e-commerce product data expert. Generate a complete product JSON based on the following analysis.

{{analysis_sections}}

Return product JSON with fields:
{
  "title": "concise SEO-friendly product title",
  "category": ["primary category", "secondary category"],
  "attributes": {"key": "value"},
  "selling_points": ["point 1", "point 2", "point 3"],
  "seo_keywords": ["keyword 1", "keyword 2"],
  "description": "detailed product description"
}

Rules:
- Prefer title, specs, and price context from scraped 1688 data when available.
- Merge scraped specs into attributes and technical details naturally.
- Keep the output consistent with the analyzed product type and features.
- Return JSON only.`
	fallback = strings.Replace(fallback, "{{analysis_sections}}", buildProductJSONAnalysisSections(analysis), 1)

	return buildProductJSONPrompt(analysis, fallback)
}

func (g *jsonGenerator) fallbackFromAnalysis(analysis *productenrich.ProductAnalysis) *productenrich.ProductJSON {
	result := &productenrich.ProductJSON{
		Category:   []string{"General", "Product"},
		Attributes: make(map[string]string),
	}

	if analysis.Representation != nil {
		result.Title = analysis.Representation.ProductType
		result.SellingPoints = analysis.Representation.Features
		for k, v := range analysis.Representation.Attributes {
			result.Attributes[k] = v
		}
	}
	if analysis.TextAttributes != nil {
		if result.Title == "" {
			result.Title = analysis.TextAttributes.Title
		}
		if len(result.SellingPoints) == 0 {
			result.SellingPoints = analysis.TextAttributes.SellingPoints
		}
		if analysis.TextAttributes.Title != "" && result.Description == "" {
			result.Description = analysis.TextAttributes.Title
		}
	}
	if analysis.ScrapedData != nil {
		if result.Title == "" {
			result.Title = analysis.ScrapedData.Title
		}
		if len(result.Category) == 0 || (len(result.Category) == 2 && result.Category[0] == "General" && result.Category[1] == "Product") {
			if categoryPath := normalizeScrapedCategoryPath(analysis.ScrapedData.Category); len(categoryPath) > 0 {
				result.Category = append([]string(nil), categoryPath...)
			}
		}
		if len(result.VariantDimensions) == 0 && len(analysis.ScrapedData.VariantDimensions) > 0 {
			result.VariantDimensions = append([]canonical.ScrapedVariantDimension(nil), analysis.ScrapedData.VariantDimensions...)
		}
		for k, v := range analysis.ScrapedData.Specs {
			if _, exists := result.Attributes[k]; !exists {
				result.Attributes[k] = v
			}
		}
		if analysis.ScrapedData.Description != "" && result.Description == "" {
			result.Description = analysis.ScrapedData.Description
		}
		if len(result.Images) == 0 && len(analysis.ScrapedData.Images) > 0 {
			result.Images = append(result.Images, analysis.ScrapedData.Images...)
		}
	}
	if result.Title == "" {
		result.Title = "Product"
	}
	if result.Description == "" {
		result.Description = fmt.Sprintf("%s generated from fallback analysis.", result.Title)
	}

	return result
}

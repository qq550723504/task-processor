package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	var sb strings.Builder

	sb.WriteString("You are an e-commerce product data expert. Based on the following product analysis, generate a complete product listing in JSON format.\n\n")

	if analysis.Representation != nil {
		repJSON, _ := json.Marshal(analysis.Representation)
		sb.WriteString(fmt.Sprintf("Product analysis:\n%s\n\n", string(repJSON)))
	}
	if analysis.TextAttributes != nil {
		textJSON, _ := json.Marshal(analysis.TextAttributes)
		sb.WriteString(fmt.Sprintf("Text attributes:\n%s\n\n", string(textJSON)))
	}
	if analysis.ImageAttributes != nil {
		imgJSON, _ := json.Marshal(analysis.ImageAttributes)
		sb.WriteString(fmt.Sprintf("Image attributes:\n%s\n\n", string(imgJSON)))
	}

	sb.WriteString(`Generate the product JSON with these fields:
{
  "title": "concise, SEO-friendly product title (max 80 chars)",
  "category": ["primary category", "sub category"],
  "attributes": {"key": "value"},
  "selling_points": ["point1", "point2", "point3"],
  "seo_keywords": ["keyword1", "keyword2"],
  "description": "detailed product description (100-300 chars)"
}

Rules:
- title must be specific and descriptive
- category should reflect the product hierarchy
- selling_points should highlight key benefits (3-5 points)
- seo_keywords should include product type, material, use case
- Only return the JSON object, no additional text.`)

	return sb.String()
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
	if result.Title == "" {
		result.Title = "Product"
	}
	if result.Description == "" {
		result.Description = fmt.Sprintf("%s generated from fallback analysis.", result.Title)
	}

	return result
}

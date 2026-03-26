package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/jsonx"
	productenrich "task-processor/internal/productenrich"

	"github.com/sirupsen/logrus"
)

type variantGenerator struct {
	llmManager productenrich.LLMManager
}

func NewVariantGenerator(llmManager productenrich.LLMManager) (productenrich.VariantGenerator, error) {
	if llmManager == nil {
		return nil, fmt.Errorf("llm manager cannot be nil")
	}

	return &variantGenerator{llmManager: llmManager}, nil
}

func (v *variantGenerator) GenerateSpecs(ctx context.Context, analysis *productenrich.ProductAnalysis) (*productenrich.ProductSpecs, error) {
	if analysis == nil {
		return nil, fmt.Errorf("analysis cannot be nil")
	}

	logger.GetGlobalLogger("productenrich/variant.go").Info("generating product specifications")

	prompt := "Extract product specifications from the following information:\n\n"
	if analysis.Representation != nil {
		repJSON, _ := json.Marshal(analysis.Representation)
		prompt += fmt.Sprintf("Product: %s\n\n", string(repJSON))
	}
	prompt += `Generate specifications in JSON format:
{
  "dimensions": {
    "length": 0.0,
    "width": 0.0,
    "height": 0.0,
    "unit": "cm"
  },
  "weight": {
    "value": 0.0,
    "unit": "kg"
  },
  "package": {
    "dimensions": {
      "length": 0.0,
      "width": 0.0,
      "height": 0.0,
      "unit": "cm"
    },
    "weight": {
      "value": 0.0,
      "unit": "kg"
    },
    "quantity": 1
  },
  "technical": {
    "material": "value",
    "power": "value",
    "voltage": "value"
  }
}

If information is not available, omit the field or use null.
Only return the JSON object, no additional text.`

	fastClient, err := v.llmManager.GetClient("fast")
	if err != nil {
		fastClient = v.llmManager.GetDefaultClient()
	}

	response, err := fastClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate specs: %w", err)
	}

	var specs productenrich.ProductSpecs
	if err := json.Unmarshal([]byte(jsonx.CleanLLMResponse(response)), &specs); err != nil {
		logrus.WithError(err).Warn("failed to parse specs JSON")
		return nil, nil
	}

	return &specs, nil
}

func (v *variantGenerator) GenerateVariants(ctx context.Context, analysis *productenrich.ProductAnalysis) ([]productenrich.ProductVariant, error) {
	if analysis == nil {
		return nil, fmt.Errorf("analysis cannot be nil")
	}

	logger.GetGlobalLogger("productenrich/variant.go").Info("generating product variants")

	prompt := "Identify product variants (SKUs) based on the following information:\n\n"
	if analysis.Representation != nil {
		repJSON, _ := json.Marshal(analysis.Representation)
		prompt += fmt.Sprintf("Product: %s\n\n", string(repJSON))
	}
	if analysis.ImageAttributes != nil {
		imageJSON, _ := json.Marshal(analysis.ImageAttributes)
		prompt += fmt.Sprintf("Image attributes: %s\n\n", string(imageJSON))
	}

	prompt += `Generate variants in JSON array format:
[
  {
    "sku": "PROD-001-RED-M",
    "attributes": {
      "color": "Red",
      "size": "M"
    },
    "price": {
      "currency": "CNY",
      "amount": 29.99,
      "compare_at": 39.99
    },
    "stock": 100,
    "images": [],
    "is_default": true
  }
]

Rules:
1. Generate variants based on color, size, style, or other distinguishing attributes
2. If no variants exist, return a single default variant
3. SKU format: PRODUCT-VARIANT-ATTRIBUTES
4. Set one variant as default (is_default: true)
5. Estimate reasonable prices in CNY (Chinese Yuan)

Only return the JSON array, no additional text.`

	defaultClient := v.llmManager.GetDefaultClient()
	response, err := defaultClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate variants: %w", err)
	}

	var variants []productenrich.ProductVariant
	if err := json.Unmarshal([]byte(jsonx.CleanLLMResponse(response)), &variants); err != nil {
		logrus.WithError(err).Warn("failed to parse variants JSON")
		return []productenrich.ProductVariant{
			{
				SKU:        "DEFAULT-001",
				Attributes: make(map[string]string),
				Stock:      0,
				IsDefault:  true,
			},
		}, nil
	}

	hasDefault := false
	for _, variant := range variants {
		if variant.IsDefault {
			hasDefault = true
			break
		}
	}
	if !hasDefault && len(variants) > 0 {
		variants[0].IsDefault = true
	}

	return variants, nil
}

func (v *variantGenerator) ExtractDimensions(ctx context.Context, text string) (*productenrich.Dimensions, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	logger.GetGlobalLogger("productenrich/variant.go").Info("extracting dimensions")
	prompt := fmt.Sprintf(`Extract product dimensions from the following text:

%s

Return in JSON format:
{
  "length": 0.0,
  "width": 0.0,
  "height": 0.0,
  "unit": "cm"
}

If dimensions are not found, return null.
Only return the JSON object, no additional text.`, text)

	var dimensions productenrich.Dimensions
	if err := v.extractWithLLM(ctx, prompt, &dimensions); err != nil {
		return nil, err
	}
	return &dimensions, nil
}

func (v *variantGenerator) ExtractWeight(ctx context.Context, text string) (*productenrich.Weight, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	logger.GetGlobalLogger("productenrich/variant.go").Info("extracting weight")
	prompt := fmt.Sprintf(`Extract product weight from the following text:

%s

Return in JSON format:
{
  "value": 0.0,
  "unit": "kg"
}

If weight is not found, return null.
Only return the JSON object, no additional text.`, text)

	var weight productenrich.Weight
	if err := v.extractWithLLM(ctx, prompt, &weight); err != nil {
		return nil, err
	}
	return &weight, nil
}

func (v *variantGenerator) extractWithLLM(ctx context.Context, prompt string, dest interface{}) error {
	fastClient, err := v.llmManager.GetClient("fast")
	if err != nil {
		fastClient = v.llmManager.GetDefaultClient()
	}

	response, err := fastClient.Generate(ctx, prompt)
	if err != nil {
		return fmt.Errorf("failed to generate: %w", err)
	}

	response = jsonx.CleanLLMResponse(response)
	if response == "null" || response == "" {
		return nil
	}

	if err := json.Unmarshal([]byte(response), dest); err != nil {
		logrus.WithError(err).Warn("failed to parse JSON response")
		return nil
	}
	return nil
}

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
		prompt += fmt.Sprintf("Representation: %s\n\n", string(repJSON))
	}
	if analysis.ScrapedData != nil {
		scrapedJSON, _ := json.Marshal(analysis.ScrapedData)
		prompt += fmt.Sprintf("1688 scraped data: %s\n\n", string(scrapedJSON))
	}
	prompt += `Return JSON:
{
  "dimensions": {"length": 0.0, "width": 0.0, "height": 0.0, "unit": "cm"},
  "weight": {"value": 0.0, "unit": "kg"},
  "package": {
    "dimensions": {"length": 0.0, "width": 0.0, "height": 0.0, "unit": "cm"},
    "weight": {"value": 0.0, "unit": "kg"},
    "quantity": 1
  },
  "technical": {"material": "value", "power": "value", "voltage": "value"}
}

Prefer values from scraped 1688 specs when available. Return JSON only.`

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
		return v.fallbackSpecsFromScraped(analysis), nil
	}

	if analysis.ScrapedData != nil {
		specs = mergeTechnicalSpecs(specs, analysis.ScrapedData.Specs)
	}
	return &specs, nil
}

func (v *variantGenerator) GenerateVariants(ctx context.Context, analysis *productenrich.ProductAnalysis) ([]productenrich.ProductVariant, error) {
	if analysis == nil {
		return nil, fmt.Errorf("analysis cannot be nil")
	}

	logger.GetGlobalLogger("productenrich/variant.go").Info("generating product variants")

	prompt := "Identify product variants from the following information:\n\n"
	if analysis.Representation != nil {
		repJSON, _ := json.Marshal(analysis.Representation)
		prompt += fmt.Sprintf("Representation: %s\n\n", string(repJSON))
	}
	if analysis.ImageAttributes != nil {
		imageJSON, _ := json.Marshal(analysis.ImageAttributes)
		prompt += fmt.Sprintf("Image attributes: %s\n\n", string(imageJSON))
	}
	if analysis.ScrapedData != nil {
		scrapedJSON, _ := json.Marshal(analysis.ScrapedData)
		prompt += fmt.Sprintf("1688 scraped data: %s\n\n", string(scrapedJSON))
	}

	prompt += `Return a JSON array:
[
  {
    "sku": "PROD-001-RED-M",
    "attributes": {"color": "Red", "size": "M"},
    "price": {"currency": "CNY", "amount": 29.99, "compare_at": 39.99, "cost_price": 19.99},
    "stock": 100,
    "images": [],
    "is_default": true
  }
]

Rules:
- Base variants on color, size, style, capacity, pack count, or other differentiators.
- Prefer 1688 price context when available.
- If there are no clear variants, return one default variant.
- Return JSON only.`

	defaultClient := v.llmManager.GetDefaultClient()
	response, err := defaultClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate variants: %w", err)
	}

	var variants []productenrich.ProductVariant
	if err := json.Unmarshal([]byte(jsonx.CleanLLMResponse(response)), &variants); err != nil {
		logrus.WithError(err).Warn("failed to parse variants JSON")
		return v.fallbackVariantsFromScraped(analysis), nil
	}

	applyScrapedPriceToVariants(variants, analysis.ScrapedData)
	ensureDefaultVariant(variants)
	return variants, nil
}

func (v *variantGenerator) ExtractDimensions(ctx context.Context, text string) (*productenrich.Dimensions, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	logger.GetGlobalLogger("productenrich/variant.go").Info("extracting dimensions")
	prompt := fmt.Sprintf(`Extract product dimensions from:
%s

Return JSON:
{"length": 0.0, "width": 0.0, "height": 0.0, "unit": "cm"}

Return null if unavailable.`, text)

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
	prompt := fmt.Sprintf(`Extract product weight from:
%s

Return JSON:
{"value": 0.0, "unit": "kg"}

Return null if unavailable.`, text)

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

func (v *variantGenerator) fallbackSpecsFromScraped(analysis *productenrich.ProductAnalysis) *productenrich.ProductSpecs {
	if analysis == nil || analysis.ScrapedData == nil {
		return nil
	}
	if len(analysis.ScrapedData.Specs) > 0 {
		specs := &productenrich.ProductSpecs{}
		specs.Technical = make(map[string]string, len(analysis.ScrapedData.Specs))
		for k, val := range analysis.ScrapedData.Specs {
			specs.Technical[k] = val
		}
		return specs
	}
	return nil
}

func (v *variantGenerator) fallbackVariantsFromScraped(analysis *productenrich.ProductAnalysis) []productenrich.ProductVariant {
	variant := productenrich.ProductVariant{
		SKU:        "DEFAULT-001",
		Attributes: make(map[string]string),
		Stock:      0,
		IsDefault:  true,
	}
	if analysis != nil && analysis.ScrapedData != nil && analysis.ScrapedData.Price > 0 {
		variant.Price = &productenrich.PriceInfo{
			Currency:  "CNY",
			Amount:    analysis.ScrapedData.Price,
			CostPrice: analysis.ScrapedData.Price,
		}
	}
	return []productenrich.ProductVariant{variant}
}

func mergeTechnicalSpecs(specs productenrich.ProductSpecs, scraped map[string]string) productenrich.ProductSpecs {
	if len(scraped) == 0 {
		return specs
	}
	if specs.Technical == nil {
		specs.Technical = make(map[string]string)
	}
	for k, v := range scraped {
		if _, exists := specs.Technical[k]; !exists {
			specs.Technical[k] = v
		}
	}
	return specs
}

func applyScrapedPriceToVariants(variants []productenrich.ProductVariant, scraped *productenrich.ScrapedData) {
	if scraped == nil || scraped.Price <= 0 {
		return
	}
	for i := range variants {
		if variants[i].Price == nil {
			variants[i].Price = &productenrich.PriceInfo{}
		}
		if variants[i].Price.Amount <= 0 {
			variants[i].Price.Amount = scraped.Price
		}
		if variants[i].Price.CostPrice <= 0 {
			variants[i].Price.CostPrice = scraped.Price
		}
		if variants[i].Price.Currency == "" {
			variants[i].Price.Currency = "CNY"
		}
	}
}

func ensureDefaultVariant(variants []productenrich.ProductVariant) {
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
}

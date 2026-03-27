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

	prompt := "从以下信息中提取产品规格：\n\n"
	if analysis.Representation != nil {
		repJSON, _ := json.Marshal(analysis.Representation)
		prompt += fmt.Sprintf("产品：%s\n\n", string(repJSON))
	}
	prompt += `以 JSON 格式生成规格：
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

如果信息不可用，省略该字段或使用 null。
只返回 JSON 对象，不要额外文本。`

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

	prompt := "根据以下信息识别产品变体（SKU）：\n\n"
	if analysis.Representation != nil {
		repJSON, _ := json.Marshal(analysis.Representation)
		prompt += fmt.Sprintf("产品：%s\n\n", string(repJSON))
	}
	if analysis.ImageAttributes != nil {
		imageJSON, _ := json.Marshal(analysis.ImageAttributes)
		prompt += fmt.Sprintf("图片属性：%s\n\n", string(imageJSON))
	}

	prompt += `以 JSON 数组格式生成变体：
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

规则：
1. 基于颜色、尺寸、款式或其他区分属性生成变体
2. 如果没有变体，返回一个默认变体
3. SKU 格式：产品 - 变体 - 属性
4. 设置一个变体为默认 (is_default: true)
5. 以 CNY（人民币）估算合理价格

只返回 JSON 数组，不要额外文本。`

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
	prompt := fmt.Sprintf(`从以下文本中提取产品尺寸：

%s

以 JSON 格式返回：
{
  "length": 0.0,
  "width": 0.0,
  "height": 0.0,
  "unit": "cm"
}

如果未找到尺寸，返回 null。
只返回 JSON 对象，不要额外文本。`, text)

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
	prompt := fmt.Sprintf(`从以下文本中提取产品重量：

%s

以 JSON 格式返回：
{
  "value": 0.0,
  "unit": "kg"
}

如果未找到重量，返回 null。
只返回 JSON 对象，不要额外文本。`, text)

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

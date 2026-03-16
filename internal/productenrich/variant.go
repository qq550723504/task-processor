// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"


	"github.com/sirupsen/logrus"
)

// VariantGenerator 变体生成器接口
type VariantGenerator interface {
	// GenerateSpecs 生成产品规格
	GenerateSpecs(ctx context.Context, analysis *ProductAnalysis) (*ProductSpecs, error)
	// GenerateVariants 生成产品变体
	GenerateVariants(ctx context.Context, analysis *ProductAnalysis) ([]ProductVariant, error)
	// ExtractDimensions 提取尺寸信息
	ExtractDimensions(ctx context.Context, text string) (*Dimensions, error)
	// ExtractWeight 提取重量信息
	ExtractWeight(ctx context.Context, text string) (*Weight, error)
}

// variantGenerator 变体生成器实现
type variantGenerator struct {
	llmManager LLMManager
}

// NewVariantGenerator 创建新的变体生成器
func NewVariantGenerator(llmManager LLMManager) (VariantGenerator, error) {
	if llmManager == nil {
		return nil, fmt.Errorf("llm manager cannot be nil")
	}

	return &variantGenerator{
		llmManager: llmManager,
	}, nil
}

// GenerateSpecs 生成产品规格
func (v *variantGenerator) GenerateSpecs(ctx context.Context, analysis *ProductAnalysis) (*ProductSpecs, error) {
	if analysis == nil {
		return nil, fmt.Errorf("analysis cannot be nil")
	}

	logrus.Info("generating product specifications")

	// 构建提示词
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

	// 使用快速客户端生成规格
	fastClient, err := v.llmManager.GetClient("fast")
	if err != nil {
		fastClient = v.llmManager.GetDefaultClient()
	}

	response, err := fastClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate specs: %w", err)
	}

	// 解析响应
	var specs ProductSpecs
	if err := json.Unmarshal([]byte(response), &specs); err != nil {
		logrus.WithError(err).Warn("failed to parse specs JSON")
		return nil, nil // 返回 nil 表示没有规格信息
	}

	return &specs, nil
}

// GenerateVariants 生成产品变体
func (v *variantGenerator) GenerateVariants(ctx context.Context, analysis *ProductAnalysis) ([]ProductVariant, error) {
	if analysis == nil {
		return nil, fmt.Errorf("analysis cannot be nil")
	}

	logrus.Info("generating product variants")

	// 构建提示词
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
      "currency": "USD",
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
5. Estimate reasonable prices in USD

Only return the JSON array, no additional text.`

	// 使用默认客户端生成变体
	defaultClient := v.llmManager.GetDefaultClient()

	response, err := defaultClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate variants: %w", err)
	}

	// 解析响应
	var variants []ProductVariant
	if err := json.Unmarshal([]byte(response), &variants); err != nil {
		logrus.WithError(err).Warn("failed to parse variants JSON")
		// 返回默认变体
		return []ProductVariant{
			{
				SKU:        "DEFAULT-001",
				Attributes: make(map[string]string),
				Stock:      0,
				IsDefault:  true,
			},
		}, nil
	}

	// 确保至少有一个默认变体
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

// ExtractDimensions 提取尺寸信息
func (v *variantGenerator) ExtractDimensions(ctx context.Context, text string) (*Dimensions, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	logrus.Info("extracting dimensions")

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

	// 使用快速客户端
	fastClient, err := v.llmManager.GetClient("fast")
	if err != nil {
		fastClient = v.llmManager.GetDefaultClient()
	}

	response, err := fastClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to extract dimensions: %w", err)
	}

	// 检查是否返回 null
	response = strings.TrimSpace(response)
	if response == "null" || response == "" {
		return nil, nil
	}

	// 解析响应
	var dimensions Dimensions
	if err := json.Unmarshal([]byte(response), &dimensions); err != nil {
		logrus.WithError(err).Warn("failed to parse dimensions JSON")
		return nil, nil
	}

	return &dimensions, nil
}

// ExtractWeight 提取重量信息
func (v *variantGenerator) ExtractWeight(ctx context.Context, text string) (*Weight, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	logrus.Info("extracting weight")

	prompt := fmt.Sprintf(`Extract product weight from the following text:

%s

Return in JSON format:
{
  "value": 0.0,
  "unit": "kg"
}

If weight is not found, return null.
Only return the JSON object, no additional text.`, text)

	// 使用快速客户端
	fastClient, err := v.llmManager.GetClient("fast")
	if err != nil {
		fastClient = v.llmManager.GetDefaultClient()
	}

	response, err := fastClient.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to extract weight: %w", err)
	}

	// 检查是否返回 null
	response = strings.TrimSpace(response)
	if response == "null" || response == "" {
		return nil, nil
	}

	// 解析响应
	var weight Weight
	if err := json.Unmarshal([]byte(response), &weight); err != nil {
		logrus.WithError(err).Warn("failed to parse weight JSON")
		return nil, nil
	}

	return &weight, nil
}

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

	sb.WriteString("你是一位电商产品数据专家。根据以下产品分析，生成完整的产品列表 JSON 格式。\n\n")

	if analysis.Representation != nil {
		repJSON, _ := json.Marshal(analysis.Representation)
		sb.WriteString(fmt.Sprintf("产品分析：\n%s\n\n", string(repJSON)))
	}
	if analysis.TextAttributes != nil {
		textJSON, _ := json.Marshal(analysis.TextAttributes)
		sb.WriteString(fmt.Sprintf("文本属性：\n%s\n\n", string(textJSON)))
	}
	if analysis.ImageAttributes != nil {
		imgJSON, _ := json.Marshal(analysis.ImageAttributes)
		sb.WriteString(fmt.Sprintf("图片属性：\n%s\n\n", string(imgJSON)))
	}

	sb.WriteString(`生成包含以下字段的产品 JSON：
{
  "title": "简洁、SEO 友好的产品标题（最多 80 个字符）",
  "category": ["主类别", "子类别"],
  "attributes": {"键": "值"},
  "selling_points": ["卖点 1", "卖点 2", "卖点 3"],
  "seo_keywords": ["关键词 1", "关键词 2"],
  "description": "详细产品描述（100-300 个字符）"
}

规则：
- title 必须具体且具有描述性
- category 应反映产品层次结构
- selling_points 应突出关键优势（3-5 点）
- seo_keywords 应包含产品类型、材质、使用场景
- 只返回 JSON 对象，不要额外文本。`)

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

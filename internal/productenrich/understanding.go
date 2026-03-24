// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"task-processor/internal/core/logger"
	"task-processor/internal/prompt"

	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/pkg/strx"

	"github.com/sirupsen/logrus"
)

// ProductUnderstanding 产品理解接口
type ProductUnderstanding interface {
	// AnalyzeProduct 分析产品
	AnalyzeProduct(ctx context.Context, input *ParsedInput) (*ProductAnalysis, error)
	// AnalyzeImage 识别图片属性
	AnalyzeImage(ctx context.Context, imagePath string) (*ImageAttributes, error)
	// ExtractTextAttributes 提取文本属性
	ExtractTextAttributes(ctx context.Context, text string) (*TextAttributes, error)
	// FuseMultimodal 融合多模态信息
	FuseMultimodal(ctx context.Context, imageAttr *ImageAttributes, textAttr *TextAttributes) (*ProductRepresentation, error)
}

// productUnderstanding 产品理解实现
type productUnderstanding struct {
	llmManager LLMManager
}

// NewProductUnderstanding 创建新的产品理解实例
func NewProductUnderstanding(llmManager LLMManager) (ProductUnderstanding, error) {
	if llmManager == nil {
		return nil, fmt.Errorf("llm manager cannot be nil")
	}

	return &productUnderstanding{
		llmManager: llmManager,
	}, nil
}

// AnalyzeProduct 分析产品
func (p *productUnderstanding) AnalyzeProduct(ctx context.Context, input *ParsedInput) (*ProductAnalysis, error) {
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}

	analysis := &ProductAnalysis{}

	// 并发分析所有图片，合并属性
	if len(input.Images) > 0 {
		type result struct {
			attr *ImageAttributes
			err  error
		}
		results := make([]result, len(input.Images))
		var wg sync.WaitGroup
		for i, imgURL := range input.Images {
			wg.Add(1)
			go func(idx int, url string) {
				defer wg.Done()
				attr, err := p.AnalyzeImage(ctx, url)
				results[idx] = result{attr: attr, err: err}
			}(i, imgURL)
		}
		wg.Wait()

		for i, r := range results {
			if r.err != nil {
				logrus.WithError(r.err).WithField("image", input.Images[i]).Warn("failed to analyze image")
				continue
			}
			if analysis.ImageAttributes == nil {
				analysis.ImageAttributes = r.attr
				continue
			}
			// 用后续图片补充空白字段
			if analysis.ImageAttributes.Color == "" || analysis.ImageAttributes.Color == "unknown" {
				analysis.ImageAttributes.Color = r.attr.Color
			}
			if analysis.ImageAttributes.Material == "" || analysis.ImageAttributes.Material == "unknown" {
				analysis.ImageAttributes.Material = r.attr.Material
			}
			if analysis.ImageAttributes.Scene == "" || analysis.ImageAttributes.Scene == "unknown" {
				analysis.ImageAttributes.Scene = r.attr.Scene
			}
			if analysis.ImageAttributes.Usage == "" || analysis.ImageAttributes.Usage == "unknown" {
				analysis.ImageAttributes.Usage = r.attr.Usage
			}
		}
	}

	// 提取文本属性（如果有）
	if input.Text != "" {
		textAttr, err := p.ExtractTextAttributes(ctx, input.Text)
		if err != nil {
			logrus.WithError(err).Warn("failed to extract text attributes")
		} else {
			analysis.TextAttributes = textAttr
		}
	}

	// 如果有抓取的数据，也提取文本属性并合并
	if input.ScrapedData != nil && input.ScrapedData.Description != "" {
		scrapedAttr, err := p.ExtractTextAttributes(ctx, input.ScrapedData.Description)
		if err != nil {
			logrus.WithError(err).Warn("failed to extract scraped text attributes")
		} else if analysis.TextAttributes == nil {
			analysis.TextAttributes = scrapedAttr
		} else {
			// 合并：scraped 的属性补充到已有属性中（不覆盖）
			for k, v := range scrapedAttr.Attributes {
				if _, exists := analysis.TextAttributes.Attributes[k]; !exists {
					analysis.TextAttributes.Attributes[k] = v
				}
			}
			// 合并卖点（去重）
			existing := make(map[string]struct{}, len(analysis.TextAttributes.SellingPoints))
			for _, sp := range analysis.TextAttributes.SellingPoints {
				existing[sp] = struct{}{}
			}
			for _, sp := range scrapedAttr.SellingPoints {
				if _, dup := existing[sp]; !dup {
					analysis.TextAttributes.SellingPoints = append(analysis.TextAttributes.SellingPoints, sp)
				}
			}
		}
	}

	// 融合多模态信息
	if analysis.ImageAttributes != nil || analysis.TextAttributes != nil {
		representation, err := p.FuseMultimodal(ctx, analysis.ImageAttributes, analysis.TextAttributes)
		if err != nil {
			logrus.WithError(err).Error("failed to fuse multimodal information")
			return nil, err
		}
		analysis.Representation = representation
	}

	return analysis, nil
}

// AnalyzeImage 识别图片属性
func (p *productUnderstanding) AnalyzeImage(ctx context.Context, imagePath string) (*ImageAttributes, error) {
	if imagePath == "" {
		return nil, fmt.Errorf("image path cannot be empty")
	}

	logger.GetGlobalLogger("productenrich/understanding.go").WithField("path", imagePath).Info("analyzing image")

	// 构建提示词
	promptText := prompt.GlobalRegistry.Get(prompt.KProductEnrichUnderstandingAnalyzeImage, `Analyze this product image and extract the following attributes in JSON format:
{
  "color": "the main color of the product",
  "material": "the material the product is made of",
  "scene": "the scene or context where the product is shown",
  "usage": "the intended use or purpose of the product"
}

Only return the JSON object, no additional text.`)

	// 使用视觉客户端分析图片
	visionClient, err := p.llmManager.GetClient("vision")
	if err != nil {
		var fallbackErr error
		visionClient, fallbackErr = p.llmManager.GetClient("default")
		if fallbackErr != nil || visionClient == nil {
			return nil, fmt.Errorf("failed to get vision or default client: %w", err)
		}
	}

	response, err := visionClient.AnalyzeImage(ctx, imagePath, promptText)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze image: %w", err)
	}

	// 解析响应
	var attributes ImageAttributes
	if err := json.Unmarshal([]byte(jsonx.CleanLLMResponse(response)), &attributes); err != nil {
		// 如果解析失败，尝试从文本中提取
		logrus.WithError(err).Warn("failed to parse JSON response, using text extraction")

		// 简单的文本提取（实际项目中可以更复杂）
		attributes = ImageAttributes{
			Color:    "unknown",
			Material: "unknown",
			Scene:    "unknown",
			Usage:    "unknown",
		}
	}

	return &attributes, nil
}

// ExtractTextAttributes 提取文本属性
func (p *productUnderstanding) ExtractTextAttributes(ctx context.Context, text string) (*TextAttributes, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	logger.GetGlobalLogger("productenrich/understanding.go").Info("extracting text attributes")

	// 构建提示词
	promptText, promptErr := prompt.GlobalRegistry.Render(prompt.KProductEnrichUnderstandingExtractText, map[string]any{
		"Text": text,
	}, "")
	if promptErr != nil || promptText == "" {
		promptText = fmt.Sprintf(`Analyze this product description and extract the following information in JSON format:
{
  "title": "a concise product title",
  "attributes": {
    "key1": "value1",
    "key2": "value2"
  },
  "selling_points": ["point1", "point2", "point3"]
}

Product description:
%s

Only return the JSON object, no additional text.`, text)
	}

	// 使用快速客户端提取属性
	fastClient, err := p.llmManager.GetClient("fast")
	if err != nil {
		var fallbackErr error
		fastClient, fallbackErr = p.llmManager.GetClient("default")
		if fallbackErr != nil || fastClient == nil {
			return nil, fmt.Errorf("failed to get fast or default client: %w", err)
		}
	}

	response, err := fastClient.Generate(ctx, promptText)
	if err != nil {
		return nil, fmt.Errorf("failed to extract text attributes: %w", err)
	}

	// 解析响应
	var attributes TextAttributes
	if err := json.Unmarshal([]byte(jsonx.CleanLLMResponse(response)), &attributes); err != nil {
		logrus.WithError(err).Warn("failed to parse JSON response")

		// 返回默认值
		attributes = TextAttributes{
			Title:         strx.TruncateString(text, 50),
			Attributes:    make(map[string]string),
			SellingPoints: []string{},
		}
	}

	return &attributes, nil
}

// FuseMultimodal 融合多模态信息
func (p *productUnderstanding) FuseMultimodal(ctx context.Context, imageAttr *ImageAttributes, textAttr *TextAttributes) (*ProductRepresentation, error) {
	logger.GetGlobalLogger("productenrich/understanding.go").Info("fusing multimodal information")

	// 构建融合提示词
	promptPrefix := prompt.GlobalRegistry.Get(prompt.KProductEnrichUnderstandingFuseMultimodal, "Combine the following image and text attributes to create a unified product representation:")
	promptText := promptPrefix + "\n\n"

	if imageAttr != nil {
		imageJSON, _ := json.Marshal(imageAttr)
		promptText += fmt.Sprintf("Image attributes: %s\n\n", string(imageJSON))
	}

	if textAttr != nil {
		textJSON, _ := json.Marshal(textAttr)
		promptText += fmt.Sprintf("Text attributes: %s\n\n", string(textJSON))
	}

	promptText += `Generate a unified product representation in JSON format:
{
  "product_type": "the type or category of the product",
  "attributes": {
    "key1": "value1",
    "key2": "value2"
  },
  "features": ["feature1", "feature2", "feature3"]
}

Only return the JSON object, no additional text.`

	// 使用默认客户端融合信息
	defaultClient, err := p.llmManager.GetClient("default")
	if err != nil || defaultClient == nil {
		return nil, fmt.Errorf("failed to get default client: %w", err)
	}
	response, err := defaultClient.Generate(ctx, promptText)
	if err != nil {
		return nil, fmt.Errorf("failed to fuse multimodal information: %w", err)
	}

	// 解析响应
	var representation ProductRepresentation
	if err := json.Unmarshal([]byte(jsonx.CleanLLMResponse(response)), &representation); err != nil {
		logrus.WithError(err).Warn("failed to parse JSON response")

		// 创建默认表示
		representation = ProductRepresentation{
			ProductType: "unknown",
			Attributes:  make(map[string]string),
			Features:    []string{},
		}

		// 从图片和文本属性中提取信息
		if imageAttr != nil {
			representation.Attributes["color"] = imageAttr.Color
			representation.Attributes["material"] = imageAttr.Material
		}
		if textAttr != nil {
			representation.ProductType = textAttr.Title
			for k, v := range textAttr.Attributes {
				representation.Attributes[k] = v
			}
			representation.Features = textAttr.SellingPoints
		}
	}

	return &representation, nil
}

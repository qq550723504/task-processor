package enrich

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/jsonx"
	"task-processor/internal/pkg/strx"
	productenrich "task-processor/internal/productenrich"
	"task-processor/internal/prompt"

	"github.com/sirupsen/logrus"
)

type productUnderstanding struct {
	llmManager productenrich.LLMManager
}

func NewProductUnderstanding(llmManager productenrich.LLMManager) (productenrich.ProductUnderstanding, error) {
	if llmManager == nil {
		return nil, fmt.Errorf("llm manager cannot be nil")
	}

	return &productUnderstanding{llmManager: llmManager}, nil
}

func (p *productUnderstanding) AnalyzeProduct(ctx context.Context, input *productenrich.ParsedInput) (*productenrich.ProductAnalysis, error) {
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}

	analysis := &productenrich.ProductAnalysis{}

	if len(input.Images) > 0 {
		type result struct {
			attr *productenrich.ImageAttributes
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

	if input.Text != "" {
		textAttr, err := p.ExtractTextAttributes(ctx, input.Text)
		if err != nil {
			logrus.WithError(err).Warn("failed to extract text attributes")
		} else {
			analysis.TextAttributes = textAttr
		}
	}

	if input.ScrapedData != nil && input.ScrapedData.Description != "" {
		scrapedAttr, err := p.ExtractTextAttributes(ctx, input.ScrapedData.Description)
		if err != nil {
			logrus.WithError(err).Warn("failed to extract scraped text attributes")
		} else if analysis.TextAttributes == nil {
			analysis.TextAttributes = scrapedAttr
		} else {
			if analysis.TextAttributes.Attributes == nil {
				analysis.TextAttributes.Attributes = make(map[string]string)
			}
			if scrapedAttr.Attributes == nil {
				scrapedAttr.Attributes = make(map[string]string)
			}
			for k, v := range scrapedAttr.Attributes {
				if _, exists := analysis.TextAttributes.Attributes[k]; !exists {
					analysis.TextAttributes.Attributes[k] = v
				}
			}
			existing := make(map[string]struct{}, len(analysis.TextAttributes.SellingPoints))
			for _, sp := range analysis.TextAttributes.SellingPoints {
				existing[sp] = struct{}{}
			}
			for _, sp := range scrapedAttr.SellingPoints {
				if _, dup := existing[sp]; !dup {
					analysis.TextAttributes.SellingPoints = append(analysis.TextAttributes.SellingPoints, sp)
					existing[sp] = struct{}{}
				}
			}
		}
	}

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

func (p *productUnderstanding) AnalyzeImage(ctx context.Context, imagePath string) (*productenrich.ImageAttributes, error) {
	if imagePath == "" {
		return nil, fmt.Errorf("image path cannot be empty")
	}

	logger.GetGlobalLogger("productenrich/understanding.go").WithField("path", imagePath).Info("analyzing image")

	defaultImagePrompt := `Analyze this product image and extract the following attributes in JSON format:
{
  "color": "the main color of the product",
  "material": "the material the product is made of",
  "scene": "the scene or context where the product is shown",
  "usage": "the intended use or purpose of the product"
}

Only return the JSON object, no additional text.`
	var promptText string
	if prompt.GlobalRegistry != nil {
		promptText = prompt.GlobalRegistry.Get(prompt.KProductEnrichUnderstandingAnalyzeImage, defaultImagePrompt)
	} else {
		promptText = defaultImagePrompt
	}

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

	var attributes productenrich.ImageAttributes
	if err := json.Unmarshal([]byte(jsonx.CleanLLMResponse(response)), &attributes); err != nil {
		logrus.WithError(err).Warn("failed to parse JSON response, using text extraction")
		attributes = productenrich.ImageAttributes{
			Color:    "unknown",
			Material: "unknown",
			Scene:    "unknown",
			Usage:    "unknown",
		}
	}

	return &attributes, nil
}

func (p *productUnderstanding) ExtractTextAttributes(ctx context.Context, text string) (*productenrich.TextAttributes, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	logger.GetGlobalLogger("productenrich/understanding.go").Info("extracting text attributes")

	var promptText string
	if prompt.GlobalRegistry != nil {
		var promptErr error
		promptText, promptErr = prompt.GlobalRegistry.Render(prompt.KProductEnrichUnderstandingExtractText, map[string]any{
			"Text": text,
		}, "")
		if promptErr != nil {
			promptText = ""
		}
	}
	if promptText == "" {
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

	var attributes productenrich.TextAttributes
	if err := json.Unmarshal([]byte(jsonx.CleanLLMResponse(response)), &attributes); err != nil {
		logrus.WithError(err).Warn("failed to parse JSON response")
		attributes = productenrich.TextAttributes{
			Title:         strx.TruncateString(text, 50),
			Attributes:    make(map[string]string),
			SellingPoints: []string{},
		}
	}

	return &attributes, nil
}

func (p *productUnderstanding) FuseMultimodal(ctx context.Context, imageAttr *productenrich.ImageAttributes, textAttr *productenrich.TextAttributes) (*productenrich.ProductRepresentation, error) {
	logger.GetGlobalLogger("productenrich/understanding.go").Info("fusing multimodal information")

	defaultFusePrompt := "Combine the following image and text attributes to create a unified product representation:"
	var promptPrefix string
	if prompt.GlobalRegistry != nil {
		promptPrefix = prompt.GlobalRegistry.Get(prompt.KProductEnrichUnderstandingFuseMultimodal, defaultFusePrompt)
	} else {
		promptPrefix = defaultFusePrompt
	}
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

	defaultClient, err := p.llmManager.GetClient("default")
	if err != nil || defaultClient == nil {
		return nil, fmt.Errorf("failed to get default client: %w", err)
	}
	response, err := defaultClient.Generate(ctx, promptText)
	if err != nil {
		return nil, fmt.Errorf("failed to fuse multimodal information: %w", err)
	}

	var representation productenrich.ProductRepresentation
	if err := json.Unmarshal([]byte(jsonx.CleanLLMResponse(response)), &representation); err != nil {
		logrus.WithError(err).Warn("failed to parse JSON response")
		representation = productenrich.ProductRepresentation{
			ProductType: "unknown",
			Attributes:  make(map[string]string),
			Features:    []string{},
		}
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

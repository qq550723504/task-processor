package productimage

import (
	"context"
	"fmt"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type openAICompatibleSceneGenerator struct {
	runtime *realImageComponents
	client  openaiclient.ImageGenerator
}

func NewOpenAICompatibleSceneGenerator(workDir string, client openaiclient.ImageGenerator) (SceneGenerator, error) {
	if client == nil {
		return nil, fmt.Errorf("openai-compatible image client is not configured")
	}
	rt, err := newRealImageComponents(workDir)
	if err != nil {
		return nil, err
	}
	return &openAICompatibleSceneGenerator{
		runtime: rt,
		client:  client,
	}, nil
}

func (g *openAICompatibleSceneGenerator) GenerateScene(ctx context.Context, req *SceneGenerationRequest) (*SceneGenerationResult, error) {
	if req == nil || req.SourceAsset == nil {
		return nil, fmt.Errorf("scene generation request requires source asset")
	}
	data, filename, err := g.runtime.loadAssetBytes(req.SourceAsset)
	if err != nil {
		return nil, err
	}
	response, err := g.client.EditImage(ctx, &openaiclient.ImageEditRequest{
		Model:          g.client.GetDefaultModel(),
		Prompt:         buildSceneGenerationPrompt(req),
		Image:          data,
		ImageURL:       editableAssetURL(req.SourceAsset),
		ResponseFormat: "b64_json",
		N:              1,
		Size:           "1536x1536",
	})
	if err != nil {
		return nil, err
	}
	imageData, revisedPrompt, err := decodeFirstOpenAIImage(response)
	if err != nil {
		return nil, err
	}
	optimized, err := g.runtime.processor.OptimizeForAmazon(imageData)
	if err != nil {
		return nil, err
	}
	path, info, err := g.runtime.writeProcessed(filename, "scene-model", optimized)
	if err != nil {
		return nil, err
	}
	metadata := map[string]string{
		"provider":        "openai_compatible",
		"model_family":    g.client.GetDefaultModel(),
		"generation_mode": "scene_generation",
		"prompt_ref":      req.PromptRef,
		"scene_intent":    req.SceneIntent,
		"local_path":      path,
		"format":          info.Format,
		"scene_mode":      "model",
	}
	if revisedPrompt != "" {
		metadata["revised_prompt"] = revisedPrompt
	}
	asset := ImageAsset{
		URL:        path,
		Type:       AssetTypeGalleryImage,
		SourceURL:  req.SourceAsset.SourceURL,
		Width:      info.Width,
		Height:     info.Height,
		Operations: []string{"render_scene_model"},
		Metadata:   metadata,
	}
	return &SceneGenerationResult{
		Assets: []ImageAsset{asset},
		Metadata: &GenerationMetadata{
			Provider:       "openai_compatible",
			ModelFamily:    g.client.GetDefaultModel(),
			GenerationMode: "scene_generation",
			PromptRef:      req.PromptRef,
		},
	}, nil
}

func buildSceneGenerationPrompt(req *SceneGenerationRequest) string {
	productType := ""
	title := ""
	if req.ProductContext != nil {
		productType = strings.TrimSpace(req.ProductContext.ProductType)
		title = strings.TrimSpace(req.ProductContext.Title)
	}
	base := "Create a polished ecommerce lifestyle scene around this product. Preserve the exact product identity, proportions, texture, and color. Do not replace the item."
	if req.SceneIntent != "" {
		base += " Scene intent: " + req.SceneIntent + "."
	}
	if productType != "" {
		base += " Product type: " + productType + "."
	}
	if title != "" {
		base += " Product title: " + title + "."
	}
	base += " Produce a premium marketplace-ready gallery image with clean composition and no overlaid text."
	return base
}

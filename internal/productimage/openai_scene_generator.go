package productimage

import (
	"context"
	"fmt"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/prompt"
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
	options := resolveScenePromptOptions(req, req.ProductContext)
	resolvedPrompt := buildSceneGenerationResolvedPrompt(req)
	response, err := g.client.EditImage(ctx, &openaiclient.ImageEditRequest{
		Model:          g.client.GetDefaultModel(),
		Prompt:         resolvedPrompt.Text,
		Image:          data,
		ImageURL:       editableAssetURL(req.SourceAsset),
		ResponseFormat: "b64_json",
		N:              1,
		Size:           "auto",
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
	metadata := applyPromptObservabilityMetadata(map[string]string{
		"provider":        "openai_compatible",
		"model_family":    g.client.GetDefaultModel(),
		"generation_mode": "scene_generation",
		"scene_intent":    req.SceneIntent,
		"local_path":      path,
		"format":          info.Format,
		"scene_mode":      "model",
	}, resolvedPrompt)
	metadata = applySceneGenerationMetadata(metadata, options)
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
		Metadata: sceneGenerationMetadataFromOptions(
			options,
			resolvedPrompt,
			"openai_compatible",
			g.client.GetDefaultModel(),
			"scene_generation",
		),
	}, nil
}

func buildSceneGenerationPrompt(req *SceneGenerationRequest) string {
	return buildSceneGenerationResolvedPrompt(req).Text
}

func buildSceneGenerationResolvedPrompt(req *SceneGenerationRequest) resolvedProductImagePrompt {
	productType := ""
	title := ""
	if req.ProductContext != nil {
		productType = strings.TrimSpace(req.ProductContext.ProductType)
		title = strings.TrimSpace(req.ProductContext.Title)
	}
	options := resolveScenePromptOptions(req, req.ProductContext)
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
	if options.SceneStyle != "" {
		base += " Scene style: " + options.SceneStyle + "."
	}
	if options.BackgroundTone != "" {
		base += " Background tone: " + options.BackgroundTone + "."
	}
	if options.Composition != "" {
		base += " Composition: " + options.Composition + "."
	}
	if options.PropsLevel != "" {
		base += " Props level: " + options.PropsLevel + "."
	}
	if options.AudienceHint != "" {
		base += " Audience hint: " + options.AudienceHint + "."
	}
	if options.CustomSceneHint != "" {
		base += " Custom scene hint: " + options.CustomSceneHint + "."
	}
	base += " Produce a premium marketplace-ready gallery image with clean composition and no overlaid text."
	vars := map[string]any{
		"product_type":      productType,
		"title":             title,
		"scene_intent":      strings.TrimSpace(req.SceneIntent),
		"scene_category":    options.Category,
		"scene_style":       options.SceneStyle,
		"background_tone":   options.BackgroundTone,
		"composition":       options.Composition,
		"props_level":       options.PropsLevel,
		"audience_hint":     options.AudienceHint,
		"custom_scene_hint": options.CustomSceneHint,
	}
	resolved := resolveSceneGenerationPrompt(req, vars, base, options)
	if strings.TrimSpace(resolved.Text) == "" {
		resolved.Text = base
	}
	return resolved
}

func resolveSceneGenerationPrompt(req *SceneGenerationRequest, vars map[string]any, fallback string, options scenePromptOptions) resolvedProductImagePrompt {
	for _, key := range resolveScenePromptCandidateKeys(req, options) {
		resolved := resolveProductImagePrompt(key, prompt.KProductImageSceneDefault, vars, fallback)
		if resolved.Source == "registry" {
			return resolved
		}
	}
	return resolveProductImagePrompt("", prompt.KProductImageSceneDefault, vars, fallback)
}

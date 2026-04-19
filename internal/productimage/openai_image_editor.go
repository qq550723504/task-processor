package productimage

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/prompt"
)

type openAICompatibleFaithfulEditor struct {
	runtime *realImageComponents
	client  openaiclient.ImageGenerator
}

func NewOpenAICompatibleFaithfulEditor(workDir string, client openaiclient.ImageGenerator) (FaithfulEditor, error) {
	if client == nil {
		return nil, fmt.Errorf("openai-compatible image client is not configured")
	}
	rt, err := newRealImageComponents(workDir)
	if err != nil {
		return nil, err
	}
	return &openAICompatibleFaithfulEditor{
		runtime: rt,
		client:  client,
	}, nil
}

func (e *openAICompatibleFaithfulEditor) Edit(ctx context.Context, req *FaithfulEditRequest) (*FaithfulEditResult, error) {
	if req == nil || req.SourceAsset == nil {
		return nil, fmt.Errorf("faithful edit request requires source asset")
	}
	data, filename, err := e.runtime.loadAssetBytes(req.SourceAsset)
	if err != nil {
		return nil, err
	}
	prompt := buildFaithfulEditPrompt(req)
	response, err := e.client.EditImage(ctx, &openaiclient.ImageEditRequest{
		Model:          e.client.GetDefaultModel(),
		Prompt:         prompt,
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
	optimized, err := e.runtime.processor.OptimizeForAmazon(imageData)
	if err != nil {
		return nil, err
	}
	stageName := "faithful-edit"
	assetType := AssetTypeSubjectCutout
	operationMode := "faithful_edit"
	normalizedPromptRef := normalizedFaithfulEditPromptRef(req)
	if req.Operation == "extract_subject" {
		stageName = "subject-model"
		operationMode = "extract_subject"
	} else if req.Operation == "render_white_background" {
		stageName = "white-bg-model"
		assetType = AssetTypeWhiteBgImage
		operationMode = "render_white_background"
	}
	path, info, err := e.runtime.writeProcessed(filename, stageName, optimized)
	if err != nil {
		return nil, err
	}
	metadata := map[string]string{
		"provider":        "openai_compatible",
		"model_family":    e.client.GetDefaultModel(),
		"generation_mode": operationMode,
		"prompt_ref":      normalizedPromptRef,
		"local_path":      path,
		"format":          info.Format,
	}
	if revisedPrompt != "" {
		metadata["revised_prompt"] = revisedPrompt
	}
	if req.Operation == "render_white_background" {
		metadata["background"] = "white"
		metadata["background_mode"] = "model"
	}
	return &FaithfulEditResult{
		Asset: &ImageAsset{
			URL:        path,
			Type:       assetType,
			SourceURL:  req.SourceAsset.SourceURL,
			Width:      info.Width,
			Height:     info.Height,
			Operations: []string{"render_image_model"},
			Metadata:   metadata,
		},
		Metadata: &GenerationMetadata{
			Provider:       "openai_compatible",
			ModelFamily:    e.client.GetDefaultModel(),
			GenerationMode: operationMode,
			PromptRef:      normalizedPromptRef,
		},
	}, nil
}

func editableAssetURL(asset *ImageAsset) string {
	if asset == nil {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(asset.URL)), "http://") || strings.HasPrefix(strings.ToLower(strings.TrimSpace(asset.URL)), "https://") {
		return strings.TrimSpace(asset.URL)
	}
	return strings.TrimSpace(asset.SourceURL)
}

func buildFaithfulEditPrompt(req *FaithfulEditRequest) string {
	productType := ""
	if req.ProductContext != nil {
		productType = strings.TrimSpace(req.ProductContext.ProductType)
	}
	switch req.Operation {
	case "extract_subject":
		fallback := "Isolate the product as the main subject in a clean ecommerce edit. Preserve the exact identity, shape, texture, and color. Keep the output simple and product-focused with no extra objects or text."
		if productType != "" {
			fallback = fmt.Sprintf("Isolate the %s as the main subject in a clean ecommerce edit. Preserve the exact identity, shape, texture, and color. Keep the output simple and product-focused with no extra objects or text.", productType)
		}
		return renderProductImagePrompt(req.PromptRef, prompt.KProductImageSubjectExtract, map[string]any{
			"product_type": productType,
			"title":        productTitle(req.ProductContext),
			"operation":    req.Operation,
		}, fallback)
	case "render_white_background":
		fallback := "Place the product on a plain white ecommerce background. Preserve the exact identity, proportions, texture, and color. Keep the image clean, natural, and free of extra text or objects."
		if productType != "" {
			fallback = fmt.Sprintf("Place the %s on a plain white ecommerce background. Preserve the exact identity, proportions, texture, and color. Keep the image clean, natural, and free of extra text or objects.", productType)
		}
		return renderProductImagePrompt(req.PromptRef, prompt.KProductImageWhiteBackgroundDefault, map[string]any{
			"product_type": productType,
			"title":        productTitle(req.ProductContext),
			"operation":    req.Operation,
		}, fallback)
	default:
		return "Edit this product image faithfully for ecommerce use. Preserve identity and remove irrelevant background elements."
	}
}

func productTitle(context *ProductContext) string {
	if context == nil {
		return ""
	}
	return strings.TrimSpace(context.Title)
}

func decodeFirstOpenAIImage(response *openaiclient.ImageResponse) ([]byte, string, error) {
	if response == nil || len(response.Data) == 0 {
		return nil, "", fmt.Errorf("image response contained no data")
	}
	first := response.Data[0]
	if first.B64JSON == "" {
		return nil, first.RevisedPrompt, fmt.Errorf("image response missing b64_json payload")
	}
	decoded, err := base64.StdEncoding.DecodeString(first.B64JSON)
	if err != nil {
		return nil, first.RevisedPrompt, fmt.Errorf("decode image payload: %w", err)
	}
	return decoded, first.RevisedPrompt, nil
}

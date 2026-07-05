package httpapi

import (
	"time"

	"task-processor/internal/core/config"
	grsai "task-processor/internal/infra/clients/grsai"
	openaiclient "task-processor/internal/infra/clients/openai"
	productenrich "task-processor/internal/productenrich"
	productimage "task-processor/internal/productimage"
)

func buildModelProvider(cfg *config.Config, llmMgr productenrich.LLMManager, openaiMgr *openaiclient.Manager, imageWorkDir string) (productimage.ProductImageModelProvider, error) {
	if cfg == nil {
		return nil, nil
	}

	var faithfulEditor productimage.FaithfulEditor
	var sceneGenerator productimage.SceneGenerator
	if imageCfg, ok := cfg.OpenAI.Clients["image"]; ok && imageCfg.APIStyle == "nanobanana" {
		imageClient := grsai.NewClient(grsai.Config{
			APIKey:       firstNonEmpty(imageCfg.APIKey, cfg.OpenAI.APIKey),
			Model:        imageCfg.Model,
			SubmitURL:    firstNonEmpty(imageCfg.BaseURL, cfg.OpenAI.BaseURL),
			PollInterval: time.Second,
			Timeout:      time.Duration(firstNonZero(imageCfg.Timeout, cfg.OpenAI.Timeout)) * time.Second,
		})
		editor, err := productimage.NewOpenAICompatibleFaithfulEditor(imageWorkDir, imageClient)
		if err != nil {
			return nil, err
		}
		generator, err := productimage.NewOpenAICompatibleSceneGenerator(imageWorkDir, imageClient)
		if err != nil {
			return nil, err
		}
		faithfulEditor = editor
		sceneGenerator = generator
	} else if openaiMgr != nil {
		if imageClient, err := openaiMgr.GetImageClient("image"); err == nil && imageClient != nil {
			editor, err := productimage.NewOpenAICompatibleFaithfulEditor(imageWorkDir, imageClient)
			if err != nil {
				return nil, err
			}
			generator, err := productimage.NewOpenAICompatibleSceneGenerator(imageWorkDir, imageClient)
			if err != nil {
				return nil, err
			}
			faithfulEditor = editor
			sceneGenerator = generator
		}
	}

	var segmenter productimage.SegmentationClient
	if faithfulEditor == nil && cfg.ProductImage.Segmenter.Enabled && cfg.ProductImage.Segmenter.Endpoint != "" {
		client, err := productimage.NewHTTPSegmentationClient(productimage.HTTPSegmentationClientConfig{
			Endpoint: cfg.ProductImage.Segmenter.Endpoint,
			APIKey:   cfg.ProductImage.Segmenter.APIKey,
			Timeout:  time.Duration(cfg.ProductImage.Segmenter.Timeout) * time.Second,
		})
		if err != nil {
			return nil, err
		}
		segmenter = client
	}

	var whiteBackground productimage.WhiteBackgroundClient
	if faithfulEditor == nil && cfg.ProductImage.WhiteBackground.Enabled && cfg.ProductImage.WhiteBackground.Endpoint != "" {
		client, err := productimage.NewHTTPWhiteBackgroundClient(productimage.HTTPWhiteBackgroundClientConfig{
			Endpoint: cfg.ProductImage.WhiteBackground.Endpoint,
			APIKey:   cfg.ProductImage.WhiteBackground.APIKey,
			Timeout:  time.Duration(cfg.ProductImage.WhiteBackground.Timeout) * time.Second,
		})
		if err != nil {
			return nil, err
		}
		whiteBackground = client
	}

	var remoteSceneGenerator productimage.SceneGenerationClient
	if sceneGenerator == nil && cfg.ProductImage.Scene.Enabled && cfg.ProductImage.Scene.Endpoint != "" {
		client, err := productimage.NewHTTPSceneGenerationClient(productimage.HTTPSceneGenerationClientConfig{
			Endpoint: cfg.ProductImage.Scene.Endpoint,
			APIKey:   cfg.ProductImage.Scene.APIKey,
			Timeout:  time.Duration(cfg.ProductImage.Scene.Timeout) * time.Second,
		})
		if err != nil {
			return nil, err
		}
		remoteSceneGenerator = client
	}

	if llmMgr == nil && faithfulEditor == nil && sceneGenerator == nil && segmenter == nil && whiteBackground == nil && remoteSceneGenerator == nil {
		return nil, nil
	}

	var reviewModel productimage.ImageReviewModel
	if llmMgr != nil {
		model, err := productimage.NewLLMReviewModel(llmMgr)
		if err != nil {
			return nil, err
		}
		reviewModel = model
	}

	if faithfulEditor != nil || sceneGenerator != nil {
		return productimage.NewModelProvider(faithfulEditor, sceneGenerator, reviewModel), nil
	}

	return productimage.NewDefaultModelProvider(productimage.DefaultModelProviderConfig{
		LLMManager:      llmMgr,
		WorkDir:         imageWorkDir,
		Segmenter:       segmenter,
		WhiteBackground: whiteBackground,
		SceneGenerator:  remoteSceneGenerator,
	})
}

func shouldUseModelBackedImagePipeline(provider productimage.ProductImageModelProvider) bool {
	return provider != nil && (provider.FaithfulEditor() != nil || provider.SceneGenerator() != nil || provider.ReviewModel() != nil)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

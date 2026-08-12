package httpapi

import (
	"fmt"
	"time"

	"task-processor/internal/core/config"
	grsai "task-processor/internal/infra/clients/grsai"
	openaiclient "task-processor/internal/infra/clients/openai"
	productenrich "task-processor/internal/productenrich"
	productimage "task-processor/internal/productimage"
)

type remoteImageServiceOptions struct {
	endpoint string
	apiKey   string
	timeout  int
}

type nanobananaImageClientOptions struct {
	apiKey  string
	model   string
	baseURL string
	timeout int
}

type modelProviderOptions struct {
	sceneGovernanceEnabled bool
	nanobanana             *nanobananaImageClientOptions
	segmenter              *remoteImageServiceOptions
	whiteBackground        *remoteImageServiceOptions
	scene                  *remoteImageServiceOptions
}

func newModelProviderOptions(cfg *config.Config) modelProviderOptions {
	if cfg == nil {
		return modelProviderOptions{}
	}

	options := modelProviderOptions{
		sceneGovernanceEnabled: cfg.AICapability.ProductImageSceneEnabled,
		segmenter:              remoteImageServiceOptionsFromConfig(cfg.ProductImage.Segmenter),
		whiteBackground:        remoteImageServiceOptionsFromConfig(cfg.ProductImage.WhiteBackground),
		scene:                  remoteImageServiceOptionsFromConfig(cfg.ProductImage.Scene),
	}
	if imageCfg, ok := cfg.OpenAI.Clients["image"]; ok && imageCfg.APIStyle == "nanobanana" {
		options.nanobanana = &nanobananaImageClientOptions{
			apiKey:  firstNonEmpty(imageCfg.APIKey, cfg.OpenAI.APIKey),
			model:   imageCfg.Model,
			baseURL: firstNonEmpty(imageCfg.BaseURL, cfg.OpenAI.BaseURL),
			timeout: firstNonZero(imageCfg.Timeout, cfg.OpenAI.Timeout),
		}
	}
	return options
}

func remoteImageServiceOptionsFromConfig(cfg config.ProductImageModelConfig) *remoteImageServiceOptions {
	if !cfg.Enabled || cfg.Endpoint == "" {
		return nil
	}
	return &remoteImageServiceOptions{endpoint: cfg.Endpoint, apiKey: cfg.APIKey, timeout: cfg.Timeout}
}

func buildModelProvider(options modelProviderOptions, llmMgr productenrich.LLMManager, openaiMgr *openaiclient.Manager, imageWorkDir string) (productimage.ProductImageModelProvider, error) {

	var faithfulEditor productimage.FaithfulEditor
	var sceneGenerator productimage.SceneGenerator
	if options.sceneGovernanceEnabled {
		if openaiMgr == nil {
			return nil, fmt.Errorf("productimage scene governance requires a resolver-backed image client")
		}
		imageClient, err := openaiMgr.GetImageClient(productImageSceneClientName)
		if err != nil || imageClient == nil {
			if err == nil {
				err = fmt.Errorf("image client is nil")
			}
			return nil, fmt.Errorf("build productimage scene client %q: %w", productImageSceneClientName, err)
		}
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
	} else if options.nanobanana != nil {
		imageClient := grsai.NewClient(grsai.Config{
			APIKey:       options.nanobanana.apiKey,
			Model:        options.nanobanana.model,
			SubmitURL:    options.nanobanana.baseURL,
			PollInterval: time.Second,
			Timeout:      time.Duration(options.nanobanana.timeout) * time.Second,
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
	if faithfulEditor == nil && options.segmenter != nil {
		client, err := productimage.NewHTTPSegmentationClient(productimage.HTTPSegmentationClientConfig{
			Endpoint: options.segmenter.endpoint,
			APIKey:   options.segmenter.apiKey,
			Timeout:  time.Duration(options.segmenter.timeout) * time.Second,
		})
		if err != nil {
			return nil, err
		}
		segmenter = client
	}

	var whiteBackground productimage.WhiteBackgroundClient
	if faithfulEditor == nil && options.whiteBackground != nil {
		client, err := productimage.NewHTTPWhiteBackgroundClient(productimage.HTTPWhiteBackgroundClientConfig{
			Endpoint: options.whiteBackground.endpoint,
			APIKey:   options.whiteBackground.apiKey,
			Timeout:  time.Duration(options.whiteBackground.timeout) * time.Second,
		})
		if err != nil {
			return nil, err
		}
		whiteBackground = client
	}

	var remoteSceneGenerator productimage.SceneGenerationClient
	if sceneGenerator == nil && options.scene != nil {
		client, err := productimage.NewHTTPSceneGenerationClient(productimage.HTTPSceneGenerationClientConfig{
			Endpoint: options.scene.endpoint,
			APIKey:   options.scene.apiKey,
			Timeout:  time.Duration(options.scene.timeout) * time.Second,
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

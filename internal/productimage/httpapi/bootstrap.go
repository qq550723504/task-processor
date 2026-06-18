package httpapi

import (
	"fmt"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/httpbootstrap"
	"task-processor/internal/infra/worker"
	productimage "task-processor/internal/productimage"
	productimagepipeline "task-processor/internal/productimage/pipeline"
)

type Module struct {
	Handler               RouteHandler
	Pool                  worker.WorkerPool
	Service               productimage.Service
	Closers               []func() error
	SubjectExtractor      productimage.SubjectExtractor
	WhiteBackgroundRender productimage.WhiteBackgroundRenderer
	SceneRenderer         productimage.SceneRenderer
}

func BuildModule(input BuildModuleInput) (*Module, error) {
	sourceParser, err := productimage.NewSourceParser(input.InputParser)
	if err != nil {
		return nil, fmt.Errorf("create image source parser: %w", err)
	}
	contextAnalyzer, err := productimage.NewProductContextAnalyzer(input.Understanding)
	if err != nil {
		return nil, fmt.Errorf("create image context analyzer: %w", err)
	}

	imageRepo, closers, err := buildTaskRepository(input.Config, input.Logger)
	if err != nil {
		return nil, err
	}

	imageInspector, err := productimage.NewDownloadedImageInspector(input.ImageWorkDir)
	if err != nil {
		return nil, fmt.Errorf("create downloaded image inspector: %w", err)
	}
	imageCleaner, err := productimage.NewWatermarkAwareImageCleaner(input.ImageWorkDir, input.Config.Watermark, input.Logger)
	if err != nil {
		return nil, fmt.Errorf("create watermark-aware image cleaner: %w", err)
	}
	modelProvider, err := buildModelProvider(input.Config, input.LLMManager, input.OpenAIManager, input.ImageWorkDir)
	if err != nil {
		return nil, fmt.Errorf("create productimage model provider: %w", err)
	}

	var subjectExtractor productimage.SubjectExtractor
	var whiteBgRenderer productimage.WhiteBackgroundRenderer
	var sceneRenderer productimage.SceneRenderer
	if !shouldUseModelBackedImagePipeline(modelProvider) || modelProvider.FaithfulEditor() == nil {
		subjectExtractor, err = buildSubjectExtractor(input.Config, input.ImageWorkDir)
		if err != nil {
			return nil, fmt.Errorf("create subject extractor: %w", err)
		}
		whiteBgRenderer, err = buildWhiteBackgroundRenderer(input.Config, input.ImageWorkDir)
		if err != nil {
			return nil, fmt.Errorf("create white background renderer: %w", err)
		}
	}
	if modelProvider == nil || modelProvider.SceneGenerator() == nil {
		if !shouldUseModelBackedImagePipeline(modelProvider) {
			sceneRenderer, err = buildSceneRenderer(input.ImageWorkDir)
			if err != nil {
				return nil, fmt.Errorf("create scene renderer: %w", err)
			}
		}
	}
	resolvedComponents := resolveImagePipelineComponents(modelProvider, subjectExtractor, whiteBgRenderer, sceneRenderer)
	subjectExtractor = resolvedComponents.subjectExtractor
	whiteBgRenderer = resolvedComponents.whiteBgRenderer
	sceneRenderer = resolvedComponents.sceneRenderer

	imageCapabilities := productimage.StrictServiceCapabilities()
	imageSvc, err := productimage.NewService(&productimage.ServiceConfig{
		QueueName:             "product_image_tasks",
		TaskRepo:              imageRepo,
		ModelProvider:         modelProvider,
		Capabilities:          &imageCapabilities,
		SourceParser:          sourceParser,
		ContextAnalyzer:       contextAnalyzer,
		ImageInspector:        imageInspector,
		ImageRanker:           productimage.NewDefaultImageRanker(),
		SubjectExtractor:      subjectExtractor,
		ImageCleaner:          imageCleaner,
		WhiteBgRenderer:       whiteBgRenderer,
		SceneRenderer:         sceneRenderer,
		AssetPublisher:        buildAssetPublisher(input.Config, input.Logger),
		CleanupTemporaryFiles: input.Config.ProductImage.Lifecycle.CleanupTemporaryFiles,
		ReuseExistingAssets:   input.Config.ProductImage.Lifecycle.ReuseExistingAssets,
	})
	if err != nil {
		return nil, fmt.Errorf("create image service: %w", err)
	}

	imageProcessor, err := productimagepipeline.NewProcessor(imageSvc, imageRepo, input.Logger, 2)
	if err != nil {
		return nil, fmt.Errorf("create image processor: %w", err)
	}
	imagePool := httpbootstrap.NewWorkerPool(imageProcessor, input.Config)
	imageSubmitter := &httpbootstrap.PoolSubmitter{Pool: imagePool}
	imageSvc.SetTaskSubmitter(imageSubmitter)
	imageProcessor.SetTaskSubmitter(imageSubmitter)

	imageHandler, err := NewHandler(imageSvc)
	if err != nil {
		return nil, fmt.Errorf("create image handler: %w", err)
	}

	return &Module{
		Handler:               imageHandler,
		Pool:                  imagePool,
		Service:               imageSvc,
		Closers:               closers,
		SubjectExtractor:      subjectExtractor,
		WhiteBackgroundRender: whiteBgRenderer,
		SceneRenderer:         sceneRenderer,
	}, nil
}

func buildSubjectExtractor(cfg *config.Config, imageWorkDir string) (productimage.SubjectExtractor, error) {
	if cfg == nil || !cfg.ProductImage.Segmenter.Enabled || cfg.ProductImage.Segmenter.Endpoint == "" {
		return productimage.NewHybridSubjectExtractor(imageWorkDir, nil)
	}

	client, err := productimage.NewHTTPSegmentationClient(productimage.HTTPSegmentationClientConfig{
		Endpoint: cfg.ProductImage.Segmenter.Endpoint,
		APIKey:   cfg.ProductImage.Segmenter.APIKey,
		Timeout:  time.Duration(cfg.ProductImage.Segmenter.Timeout) * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return productimage.NewHybridSubjectExtractor(imageWorkDir, client)
}

func buildWhiteBackgroundRenderer(cfg *config.Config, imageWorkDir string) (productimage.WhiteBackgroundRenderer, error) {
	if cfg == nil || !cfg.ProductImage.WhiteBackground.Enabled || cfg.ProductImage.WhiteBackground.Endpoint == "" {
		return productimage.NewHybridWhiteBackgroundRenderer(imageWorkDir, nil)
	}

	client, err := productimage.NewHTTPWhiteBackgroundClient(productimage.HTTPWhiteBackgroundClientConfig{
		Endpoint: cfg.ProductImage.WhiteBackground.Endpoint,
		APIKey:   cfg.ProductImage.WhiteBackground.APIKey,
		Timeout:  time.Duration(cfg.ProductImage.WhiteBackground.Timeout) * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return productimage.NewHybridWhiteBackgroundRenderer(imageWorkDir, client)
}

func buildSceneRenderer(imageWorkDir string) (productimage.SceneRenderer, error) {
	return productimage.NewDefaultSceneRenderer(imageWorkDir)
}

type resolvedImagePipelineComponents struct {
	subjectExtractor productimage.SubjectExtractor
	whiteBgRenderer  productimage.WhiteBackgroundRenderer
	sceneRenderer    productimage.SceneRenderer
}

func resolveImagePipelineComponents(provider productimage.ProductImageModelProvider, subjectExtractor productimage.SubjectExtractor, whiteBgRenderer productimage.WhiteBackgroundRenderer, sceneRenderer productimage.SceneRenderer) resolvedImagePipelineComponents {
	if subjectExtractor == nil && provider != nil && provider.FaithfulEditor() != nil {
		subjectExtractor = productimage.NewModelSubjectExtractor(provider.FaithfulEditor())
	}
	if whiteBgRenderer == nil && provider != nil && provider.FaithfulEditor() != nil {
		whiteBgRenderer = productimage.NewModelWhiteBackgroundRenderer(provider.FaithfulEditor())
	}
	if sceneRenderer == nil && provider != nil && provider.SceneGenerator() != nil {
		sceneRenderer = productimage.NewModelSceneRenderer(provider.SceneGenerator())
	}
	return resolvedImagePipelineComponents{
		subjectExtractor: subjectExtractor,
		whiteBgRenderer:  whiteBgRenderer,
		sceneRenderer:    sceneRenderer,
	}
}

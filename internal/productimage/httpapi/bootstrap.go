package httpapi

import (
	"fmt"
	"time"

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
	AssetPublisher        productimage.AssetPublisher
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

	imageRepo, closers, err := buildTaskRepository(input.Options.database, input.Logger)
	if err != nil {
		return nil, err
	}

	imageInspector, err := productimage.NewDownloadedImageInspector(input.ImageWorkDir)
	if err != nil {
		return nil, fmt.Errorf("create downloaded image inspector: %w", err)
	}
	imageCleaner, err := productimage.NewWatermarkAwareImageCleaner(input.ImageWorkDir, input.Options.watermark, input.Logger)
	if err != nil {
		return nil, fmt.Errorf("create watermark-aware image cleaner: %w", err)
	}
	generation, err := buildGenerationCapabilities(input)
	if err != nil {
		return nil, err
	}
	modelProvider := generation.modelProvider
	subjectExtractor := generation.subjectExtractor
	whiteBgRenderer := generation.whiteBackgroundRenderer
	sceneRenderer := generation.sceneRenderer
	if generation.allowedTenants != nil {
		contextAnalyzer = &tenantAllowlistedContextAnalyzer{inner: contextAnalyzer, allowed: generation.allowedTenants}
	}

	imageCapabilities := productimage.StrictServiceCapabilities()
	assetPublisher := generation.assetPublisher
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
		AssetPublisher:        assetPublisher,
		CleanupTemporaryFiles: input.Options.cleanupTemporaryFiles,
		ReuseExistingAssets:   input.Options.reuseExistingAssets,
		// Identity is enforced by governed model stages for allowlisted tenants.
		// Keep task creation compatible with legacy callers (for example Amazon)
		// that do not enter the authenticated canary path.
		RequireAIIdentity: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create image service: %w", err)
	}

	imageProcessor, err := productimagepipeline.NewProcessor(imageSvc, imageRepo, input.Logger, 2)
	if err != nil {
		return nil, fmt.Errorf("create image processor: %w", err)
	}
	imagePool := worker.NewPoolWithConfig(imageProcessor, worker.PoolConfig{
		Concurrency:     input.Options.workerConcurrency,
		BufferSize:      input.Options.workerBufferSize,
		TaskTimeout:     15 * time.Minute,
		EnableMetrics:   true,
		ShutdownTimeout: 30 * time.Second,
	})
	imageSubmitter := &imagePoolSubmitter{pool: imagePool}
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
		AssetPublisher:        assetPublisher,
	}, nil
}

type imagePoolSubmitter struct {
	pool worker.WorkerPool
}

func (s *imagePoolSubmitter) Submit(taskID string) error {
	return s.pool.Submit(worker.WorkerJob{TaskData: taskID})
}

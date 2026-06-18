package httpapi

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/httpbootstrap"
	"task-processor/internal/infra/database"
	storageinfra "task-processor/internal/infra/storage"
	"task-processor/internal/infra/worker"
	productimage "task-processor/internal/productimage"
	productimagepipeline "task-processor/internal/productimage/pipeline"
	productimagestore "task-processor/internal/productimage/store"
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

func buildTaskRepository(cfg *config.Config, logger *logrus.Logger) (productimage.TaskRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBTaskRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("create image task repository: %w", err)
		}
		return repo, []func() error{closer}, nil
	}

	logger.Warn("database not configured, using in-memory productimage repository")
	return productimagestore.NewMemTaskRepository(), nil, nil
}

func newDBTaskRepository(cfg *config.DatabaseConfig, logger *logrus.Logger) (productimage.TaskRepository, func() error, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("database config is nil")
	}
	db, err := database.NewSharedDatabaseFromConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("database connection failed(%s:%d/%s): %w", cfg.Host, cfg.Port, cfg.Database, err)
	}
	logger.Infof("database connected: %s:%d/%s", cfg.Host, cfg.Port, cfg.Database)

	if err := db.AutoMigrate(&productimage.Task{}); err != nil {
		return nil, nil, fmt.Errorf("productimage auto-migrate failed: %w", err)
	}

	repo := productimagestore.NewTaskRepository(db)
	closer := func() error { return database.CloseSharedDatabase(cfg, db) }
	return repo, closer, nil
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

func buildAssetPublisher(cfg *config.Config, logger *logrus.Logger) productimage.AssetPublisher {
	if cfg == nil || !cfg.ProductImage.Publisher.Enabled {
		return nil
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.ProductImage.Publisher.Provider))
	switch provider {
	case "", "local":
		publisher, err := productimage.NewLocalAssetPublisher(cfg.ProductImage.Publisher.OutputDir, cfg.ProductImage.Publisher.PublicBase)
		if err != nil {
			logger.WithError(err).Warn("local image asset publisher unavailable")
			return nil
		}
		return publisher
	case "s3":
		return buildS3AssetPublisher(cfg, logger)
	case "amazon":
		publisher, err := productimage.NewAmazonAssetPublisher(cfg)
		if err != nil {
			logger.WithError(err).Warn("amazon image asset publisher unavailable")
			return nil
		}
		return publisher
	case "hybrid":
		localPublisher, err := productimage.NewLocalAssetPublisher(cfg.ProductImage.Publisher.OutputDir, cfg.ProductImage.Publisher.PublicBase)
		if err != nil {
			logger.WithError(err).Warn("hybrid local image asset publisher unavailable")
			return nil
		}
		amazonPublisher, err := productimage.NewAmazonAssetPublisher(cfg)
		if err != nil {
			logger.WithError(err).Warn("hybrid amazon image asset publisher partially unavailable")
			return localPublisher
		}
		return productimage.NewMultiAssetPublisher(localPublisher, amazonPublisher)
	default:
		logger.Warnf("unsupported image publisher provider: %s", provider)
		return nil
	}
}

func newPublisherS3Client(cfg *config.Config) (*s3.Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	s3Cfg := cfg.ProductImage.Publisher.S3
	if strings.TrimSpace(s3Cfg.Bucket) == "" {
		return nil, fmt.Errorf("productimage.publisher.s3.bucket cannot be empty")
	}
	return storageinfra.NewS3Client(storageinfra.S3ClientConfig{
		Region:          s3Cfg.Region,
		Endpoint:        s3Cfg.Endpoint,
		AccessKeyID:     s3Cfg.AccessKeyID,
		SecretAccessKey: s3Cfg.SecretAccessKey,
		UsePathStyle:    s3Cfg.UsePathStyle,
	})
}

func buildS3AssetPublisher(cfg *config.Config, logger *logrus.Logger) productimage.AssetPublisher {
	client, err := newPublisherS3Client(cfg)
	if err != nil {
		logger.WithError(err).Warn("s3 image asset publisher unavailable")
		return nil
	}

	publicBase := strings.TrimSpace(cfg.ProductImage.Publisher.PublicBase)
	if publicBase == "" {
		publicBase = storageinfra.BuildS3PublicBase(
			cfg.ProductImage.Publisher.S3.Endpoint,
			cfg.ProductImage.Publisher.S3.Bucket,
			cfg.ProductImage.Publisher.S3.UsePathStyle,
		)
	}

	uploader := storageinfra.NewS3UploaderWithOptions(client, storageinfra.S3UploaderOptions{
		Bucket:       cfg.ProductImage.Publisher.S3.Bucket,
		PublicBase:   publicBase,
		Endpoint:     cfg.ProductImage.Publisher.S3.Endpoint,
		UsePathStyle: cfg.ProductImage.Publisher.S3.UsePathStyle,
	})
	publisher, err := productimage.NewS3AssetPublisher(productimage.S3AssetPublisherConfig{
		Uploader:   uploader,
		PublicBase: publicBase,
	})
	if err != nil {
		logger.WithError(err).Warn("s3 image asset publisher unavailable")
		return nil
	}
	return publisher
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

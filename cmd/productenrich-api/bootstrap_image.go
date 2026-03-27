package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	productimage "task-processor/internal/productimage"
	productimageapi "task-processor/internal/productimage/api"
	productimagepipeline "task-processor/internal/productimage/pipeline"
	productimagestore "task-processor/internal/productimage/store"
)

func buildImageModule(logger *logrus.Logger, deps *runtimeDeps) (*imageModule, error) {
	sourceParser, err := productimage.NewSourceParser(deps.inputParser)
	if err != nil {
		return nil, fmt.Errorf("创建图片源解析器：%w", err)
	}
	contextAnalyzer, err := productimage.NewProductContextAnalyzer(deps.understanding)
	if err != nil {
		return nil, fmt.Errorf("创建图片上下文分析器：%w", err)
	}

	imageRepo, closers, err := buildImageTaskRepository(deps.cfg, logger)
	if err != nil {
		return nil, err
	}
	deps.closers = append(deps.closers, closers...)

	imageInspector, err := productimage.NewDownloadedImageInspector(deps.imageWorkDir)
	if err != nil {
		return nil, fmt.Errorf("创建下载图片检查器：%w", err)
	}
	subjectExtractor, err := buildImageSubjectExtractor(deps.cfg, deps.imageWorkDir)
	if err != nil {
		return nil, fmt.Errorf("创建主体提取器：%w", err)
	}
	imageCleaner, err := productimage.NewWatermarkAwareImageCleaner(deps.imageWorkDir, deps.cfg.Watermark, logger)
	if err != nil {
		return nil, fmt.Errorf("创建下载图片清洗器：%w", err)
	}
	whiteBgRenderer, err := buildWhiteBackgroundRenderer(deps.cfg, deps.imageWorkDir)
	if err != nil {
		return nil, fmt.Errorf("创建白底图渲染器：%w", err)
	}

	imageCapabilities := productimage.StrictServiceCapabilities()
	imageSvc, err := productimage.NewService(&productimage.ServiceConfig{
		QueueName:             "product_image_tasks",
		TaskRepo:              imageRepo,
		Capabilities:          &imageCapabilities,
		SourceParser:          sourceParser,
		ContextAnalyzer:       contextAnalyzer,
		ImageInspector:        imageInspector,
		ImageRanker:           productimage.NewDefaultImageRanker(),
		SubjectExtractor:      subjectExtractor,
		ImageCleaner:          imageCleaner,
		WhiteBgRenderer:       whiteBgRenderer,
		AssetPublisher:        buildImageAssetPublisher(deps.cfg, logger),
		CleanupTemporaryFiles: deps.cfg.ProductImage.Lifecycle.CleanupTemporaryFiles,
		ReuseExistingAssets:   deps.cfg.ProductImage.Lifecycle.ReuseExistingAssets,
	})
	if err != nil {
		return nil, fmt.Errorf("创建图片服务：%w", err)
	}

	imageProcessor, err := productimagepipeline.NewProcessor(imageSvc, imageRepo, logger, 2)
	if err != nil {
		return nil, fmt.Errorf("创建图片处理器：%w", err)
	}
	imagePool := newWorkerPool(imageProcessor, deps.cfg)
	imageSubmitter := &poolSubmitter{pool: imagePool}
	imageSvc.SetTaskSubmitter(imageSubmitter)
	imageProcessor.SetTaskSubmitter(imageSubmitter)

	imageHandler, err := productimageapi.NewImageHandler(imageSvc)
	if err != nil {
		return nil, fmt.Errorf("创建图片处理器：%w", err)
	}

	return &imageModule{handler: imageHandler, pool: imagePool}, nil
}

func buildImageTaskRepository(cfg *config.Config, logger *logrus.Logger) (productimage.TaskRepository, []func() error, error) {
	if cfg != nil && cfg.Database != nil && cfg.Database.Host != "" {
		repo, closer, err := newDBImageTaskRepository(cfg.Database, logger)
		if err != nil {
			return nil, nil, fmt.Errorf("创建图片任务仓库：%w", err)
		}
		return repo, []func() error{closer}, nil
	}
	logger.Warn("未配置数据库，使用内存 productimage 仓库")
	return productimagestore.NewMemTaskRepository(), nil, nil
}

func buildImageSubjectExtractor(cfg *config.Config, imageWorkDir string) (productimage.SubjectExtractor, error) {
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

func buildImageAssetPublisher(cfg *config.Config, logger *logrus.Logger) productimage.AssetPublisher {
	if cfg == nil || !cfg.ProductImage.Publisher.Enabled {
		return nil
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.ProductImage.Publisher.Provider))
	switch provider {
	case "", "local":
		publisher, err := productimage.NewLocalAssetPublisher(cfg.ProductImage.Publisher.OutputDir, cfg.ProductImage.Publisher.PublicBase)
		if err != nil {
			logger.WithError(err).Warn("本地图片资源发布器无法使用")
			return nil
		}
		return publisher
	case "amazon":
		publisher, err := productimage.NewAmazonAssetPublisher(cfg)
		if err != nil {
			logger.WithError(err).Warn("亚马逊图片资源发布器无法使用")
			return nil
		}
		return publisher
	case "hybrid":
		localPublisher, err := productimage.NewLocalAssetPublisher(cfg.ProductImage.Publisher.OutputDir, cfg.ProductImage.Publisher.PublicBase)
		if err != nil {
			logger.WithError(err).Warn("混合本地图片资源发布器无法使用")
			return nil
		}
		amazonPublisher, err := productimage.NewAmazonAssetPublisher(cfg)
		if err != nil {
			logger.WithError(err).Warn("混合亚马逊图片资源发布器部分无法使用")
			return localPublisher
		}
		return productimage.NewMultiAssetPublisher(localPublisher, amazonPublisher)
	default:
		logger.Warnf("不支持的图片发布器提供者：%s", provider)
		return nil
	}
}

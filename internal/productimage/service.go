package productimage

import (
	"context"
	"fmt"
)

type CapabilityMode string

const (
	CapabilityModeCompat CapabilityMode = "compat"
	CapabilityModeStrict CapabilityMode = "strict"
)

type ServiceCapabilities struct {
	Mode                      CapabilityMode
	AllowSimpleSourceParsing  bool
	AllowMissingContext       bool
	AllowDefaultAudit         bool
	AllowDefaultRanking       bool
	AllowPassThroughMainImage bool
	AllowMissingValidator     bool
}

func DefaultServiceCapabilities() ServiceCapabilities {
	return ServiceCapabilities{
		Mode:                      CapabilityModeCompat,
		AllowSimpleSourceParsing:  true,
		AllowMissingContext:       true,
		AllowDefaultAudit:         true,
		AllowDefaultRanking:       true,
		AllowPassThroughMainImage: true,
		AllowMissingValidator:     true,
	}
}

func StrictServiceCapabilities() ServiceCapabilities {
	return ServiceCapabilities{
		Mode:                      CapabilityModeStrict,
		AllowSimpleSourceParsing:  false,
		AllowMissingContext:       false,
		AllowDefaultAudit:         false,
		AllowDefaultRanking:       false,
		AllowPassThroughMainImage: false,
		AllowMissingValidator:     false,
	}
}

type Service interface {
	CreateProcessTask(ctx context.Context, req *ImageProcessRequest) (*Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
	ReviewTask(ctx context.Context, taskID string, req *ReviewTaskRequest) (*TaskResult, error)
	ProcessImages(ctx context.Context, task *Task) (*ImageProcessResult, error)
	SetTaskSubmitter(submitter TaskSubmitter)
}

type service struct {
	taskRepo              TaskRepository
	taskSubmitter         TaskSubmitter
	queueName             string
	capabilities          ServiceCapabilities
	sourceParser          SourceParser
	contextAnalyzer       ProductContextAnalyzer
	imageInspector        ImageInspector
	imageRanker           ImageRanker
	subjectExtractor      SubjectExtractor
	imageCleaner          ImageCleaner
	whiteBgRenderer       WhiteBackgroundRenderer
	assetPublisher        AssetPublisher
	sceneRenderer         SceneRenderer
	marketValidator       MarketplaceValidator
	qualityAssessor       QualityAssessor
	reviewAssessor        ReviewAssessor
	cleanupTemporaryFiles bool
	reuseExistingAssets   bool
}

type ServiceConfig struct {
	QueueName             string
	TaskRepo              TaskRepository
	TaskSubmitter         TaskSubmitter
	Capabilities          *ServiceCapabilities
	SourceParser          SourceParser
	ContextAnalyzer       ProductContextAnalyzer
	ImageInspector        ImageInspector
	ImageRanker           ImageRanker
	SubjectExtractor      SubjectExtractor
	ImageCleaner          ImageCleaner
	WhiteBgRenderer       WhiteBackgroundRenderer
	AssetPublisher        AssetPublisher
	SceneRenderer         SceneRenderer
	MarketplaceValidator  MarketplaceValidator
	QualityAssessor       QualityAssessor
	ReviewAssessor        ReviewAssessor
	CleanupTemporaryFiles bool
	ReuseExistingAssets   bool
}

func NewService(config *ServiceConfig) (Service, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.TaskRepo == nil {
		return nil, fmt.Errorf("task repository cannot be nil")
	}
	if config.QueueName == "" {
		config.QueueName = "product_image_tasks"
	}
	if config.ImageInspector == nil {
		config.ImageInspector = NewDefaultImageInspector()
	}
	if config.ImageRanker == nil {
		config.ImageRanker = NewDefaultImageRanker()
	}
	if config.SubjectExtractor == nil {
		config.SubjectExtractor = NewDefaultSubjectExtractor()
	}
	if config.ImageCleaner == nil {
		config.ImageCleaner = NewDefaultImageCleaner()
	}
	if config.WhiteBgRenderer == nil {
		config.WhiteBgRenderer = NewDefaultWhiteBackgroundRenderer()
	}
	if config.MarketplaceValidator == nil {
		config.MarketplaceValidator = NewDefaultMarketplaceValidator()
	}
	if config.QualityAssessor == nil {
		config.QualityAssessor = NewDefaultQualityAssessor()
	}
	if config.ReviewAssessor == nil {
		config.ReviewAssessor = NewDefaultReviewAssessor()
	}

	capabilities := DefaultServiceCapabilities()
	if config.Capabilities != nil {
		capabilities = *config.Capabilities
		if capabilities.Mode == "" {
			capabilities.Mode = CapabilityModeCompat
		}
	}

	return &service{
		taskRepo:              config.TaskRepo,
		taskSubmitter:         config.TaskSubmitter,
		queueName:             config.QueueName,
		capabilities:          capabilities,
		sourceParser:          config.SourceParser,
		contextAnalyzer:       config.ContextAnalyzer,
		imageInspector:        config.ImageInspector,
		imageRanker:           config.ImageRanker,
		subjectExtractor:      config.SubjectExtractor,
		imageCleaner:          config.ImageCleaner,
		whiteBgRenderer:       config.WhiteBgRenderer,
		assetPublisher:        config.AssetPublisher,
		sceneRenderer:         config.SceneRenderer,
		marketValidator:       config.MarketplaceValidator,
		qualityAssessor:       config.QualityAssessor,
		reviewAssessor:        config.ReviewAssessor,
		cleanupTemporaryFiles: config.CleanupTemporaryFiles,
		reuseExistingAssets:   config.ReuseExistingAssets,
	}, nil
}

func (s *service) SetTaskSubmitter(submitter TaskSubmitter) {
	s.taskSubmitter = submitter
}

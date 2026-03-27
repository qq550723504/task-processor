package productenrich

import (
	"context"
	"fmt"
)

type ProductService interface {
	CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error)
	GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
	ProcessProduct(ctx context.Context, task *Task) (*ProductJSON, error)
	SetTaskSubmitter(submitter TaskSubmitter)
}

type CapabilityMode string

const (
	CapabilityModeCompat CapabilityMode = "compat"
	CapabilityModeStrict CapabilityMode = "strict"
)

type ProductServiceCapabilities struct {
	Mode                           CapabilityMode
	AllowSimpleInputParsing        bool
	AllowDefaultValidationStrategy bool
	AllowSimpleAnalysis            bool
	AllowSimpleGeneration          bool
	AllowMissingResultValidator    bool
}

func DefaultProductServiceCapabilities() ProductServiceCapabilities {
	return ProductServiceCapabilities{
		Mode:                           CapabilityModeCompat,
		AllowSimpleInputParsing:        true,
		AllowDefaultValidationStrategy: true,
		AllowSimpleAnalysis:            true,
		AllowSimpleGeneration:          true,
		AllowMissingResultValidator:    true,
	}
}

func StrictProductServiceCapabilities() ProductServiceCapabilities {
	return ProductServiceCapabilities{
		Mode:                           CapabilityModeStrict,
		AllowSimpleInputParsing:        false,
		AllowDefaultValidationStrategy: false,
		AllowSimpleAnalysis:            false,
		AllowSimpleGeneration:          false,
		AllowMissingResultValidator:    false,
	}
}

type productService struct {
	taskRepo             TaskRepository
	redisClient          RedisClient
	taskSubmitter        TaskSubmitter
	queueName            string
	capabilities         ProductServiceCapabilities
	inputParser          InputParser
	productUnderstanding ProductUnderstanding
	jsonGenerator        JSONGenerator
	variantGenerator     VariantGenerator
	inputValidator       InputValidator
	qualityScorer        QualityScorer
	strategySelector     StrategySelector
	enhancementSuggester EnhancementSuggester
	resultValidator      ResultValidator
}

type ProductServiceConfig struct {
	QueueName            string
	TaskRepo             TaskRepository
	RedisClient          RedisClient
	TaskSubmitter        TaskSubmitter
	Capabilities         *ProductServiceCapabilities
	InputParser          InputParser
	ProductUnderstanding ProductUnderstanding
	JSONGenerator        JSONGenerator
	VariantGenerator     VariantGenerator
	InputValidator       InputValidator
	QualityScorer        QualityScorer
	StrategySelector     StrategySelector
	EnhancementSuggester EnhancementSuggester
	ResultValidator      ResultValidator
}

func (s *productService) SetTaskSubmitter(submitter TaskSubmitter) {
	s.taskSubmitter = submitter
}

func NewProductService(config *ProductServiceConfig) (ProductService, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.TaskRepo == nil {
		return nil, fmt.Errorf("task repository cannot be nil")
	}
	if config.RedisClient == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}

	if config.QueueName == "" {
		config.QueueName = "product_tasks"
	}

	capabilities := DefaultProductServiceCapabilities()
	if config.Capabilities != nil {
		capabilities = *config.Capabilities
		if capabilities.Mode == "" {
			capabilities.Mode = CapabilityModeCompat
		}
	}

	return &productService{
		taskRepo:             config.TaskRepo,
		redisClient:          config.RedisClient,
		taskSubmitter:        config.TaskSubmitter,
		queueName:            config.QueueName,
		capabilities:         capabilities,
		inputParser:          config.InputParser,
		productUnderstanding: config.ProductUnderstanding,
		jsonGenerator:        config.JSONGenerator,
		variantGenerator:     config.VariantGenerator,
		inputValidator:       config.InputValidator,
		qualityScorer:        config.QualityScorer,
		strategySelector:     config.StrategySelector,
		enhancementSuggester: config.EnhancementSuggester,
		resultValidator:      config.ResultValidator,
	}, nil
}

// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"fmt"
	"task-processor/internal/infra/worker"
)

// ProductService 产品服务接口
type ProductService interface {
	// CreateGenerateTask 创建产品生成任务
	CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error)
	// GetTaskResult 获取任务结果
	GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
	// ProcessProduct 处理产品生成（由 Worker 调用）
	ProcessProduct(ctx context.Context, task *Task) (*ProductJSON, error)
	// SetWorkerPool 注入 WorkerPool，解决 Pool↔Service 的循环依赖
	SetWorkerPool(pool worker.WorkerPool)
}

// productService 产品服务实现
type productService struct {
	taskRepo             TaskRepository
	redisClient          RedisClient
	workerPool           worker.WorkerPool
	queueName            string
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

// ProductServiceConfig 产品服务配置
type ProductServiceConfig struct {
	QueueName   string
	TaskRepo    TaskRepository
	RedisClient RedisClient
	// WorkerPool 用于直接提交任务，替代 Redis 队列消费模式
	WorkerPool           worker.WorkerPool
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

// SetWorkerPool 在 service 创建后注入 WorkerPool（解决循环依赖：Pool 需要 service，service 需要 Pool）。
func (s *productService) SetWorkerPool(pool worker.WorkerPool) {
	s.workerPool = pool
}

// NewProductService 创建新的产品服务。
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

	return &productService{
		taskRepo:             config.TaskRepo,
		redisClient:          config.RedisClient,
		workerPool:           config.WorkerPool,
		queueName:            config.QueueName,
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

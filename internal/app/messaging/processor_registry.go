// Package messaging 提供任务处理器注册表
package messaging

import (
	"maps"
	"sync"

	apptask "task-processor/internal/app/task"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// TaskProcessorRegistry 任务处理器注册表，负责将 worker.Processor 包装为 rabbitmq.MessageHandler。
type TaskProcessorRegistry struct {
	processors     map[string]worker.Processor
	handlers       map[string]rabbitmq.MessageHandler
	resultReporter *ResultReporter
	storeAPI       api.StoreAPI
	ownedStores    []int64
	deduplicator   *apptask.DeduplicationManager
	logger         *logrus.Logger
	mu             sync.RWMutex
}

// NewTaskProcessorRegistry 创建任务处理器注册表。
func NewTaskProcessorRegistry(
	resultReporter *ResultReporter,
	storeAPI api.StoreAPI,
	ownedStores []int64,
	deduplicator *apptask.DeduplicationManager,
	logger *logrus.Logger,
) *TaskProcessorRegistry {
	return &TaskProcessorRegistry{
		processors:     make(map[string]worker.Processor),
		handlers:       make(map[string]rabbitmq.MessageHandler),
		resultReporter: resultReporter,
		storeAPI:       storeAPI,
		ownedStores:    ownedStores,
		deduplicator:   deduplicator,
		logger:         logger,
	}
}

// RegisterProcessor 注册处理器，同时创建对应的消息处理器。
func (r *TaskProcessorRegistry) RegisterProcessor(platform string, processor worker.Processor) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.processors[platform] = processor
	r.handlers[platform] = NewTaskHandler(TaskHandlerConfig{
		Platform:       platform,
		Processor:      processor,
		ResultReporter: r.resultReporter,
		StoreAPI:       r.storeAPI,
		OwnedStores:    r.ownedStores,
		Deduplicator:   r.deduplicator,
		Logger:         r.logger,
	})

	r.logger.Infof("注册增强处理器: 平台=%s", platform)
}

// GetHandler 获取指定平台的消息处理器。
func (r *TaskProcessorRegistry) GetHandler(platform string) (rabbitmq.MessageHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[platform]
	return handler, exists
}

// GetAllHandlers 返回所有消息处理器的副本。
func (r *TaskProcessorRegistry) GetAllHandlers() map[string]rabbitmq.MessageHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := make(map[string]rabbitmq.MessageHandler, len(r.handlers))
	maps.Copy(handlers, r.handlers)
	return handlers
}

// GetQueueName 根据平台获取队列名称。
func (r *TaskProcessorRegistry) GetQueueName(platform string) string {
	return apptask.NewMessageAdapter().GetQueueName(platform)
}

// GetStats 返回注册表统计信息。
func (r *TaskProcessorRegistry) GetStats() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	platforms := make(map[string]any, len(r.processors))
	for platform := range r.processors {
		platforms[platform] = map[string]any{"status": "registered"}
	}

	stats := map[string]any{
		"total_processors": len(r.processors),
		"platforms":        platforms,
	}

	if r.resultReporter != nil {
		stats["result_reporter"] = r.resultReporter.GetStats()
	}

	return stats
}

package consumer

import (
	"maps"
	"sync"

	apptask "task-processor/internal/app/task"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// TaskProcessorRegistry stores processors and builds RabbitMQ handlers from shared components.
type TaskProcessorRegistry struct {
	processors     map[string]worker.Processor
	handlers       map[string]rabbitmq.MessageHandler
	resultReporter *ResultReporter
	storeAPI       api.StoreAPI
	ownedStores    []int64
	useStoreQueues bool
	deduplicator   *apptask.DeduplicationManager
	logger         *logrus.Logger
	mu             sync.RWMutex
}

// NewTaskProcessorRegistry creates a registry for platform processors.
func NewTaskProcessorRegistry(
	resultReporter *ResultReporter,
	storeAPI api.StoreAPI,
	ownedStores []int64,
	useStoreQueues bool,
	deduplicator *apptask.DeduplicationManager,
	logger *logrus.Logger,
) *TaskProcessorRegistry {
	return &TaskProcessorRegistry{
		processors:     make(map[string]worker.Processor),
		handlers:       make(map[string]rabbitmq.MessageHandler),
		resultReporter: resultReporter,
		storeAPI:       storeAPI,
		ownedStores:    ownedStores,
		useStoreQueues: useStoreQueues,
		deduplicator:   deduplicator,
		logger:         logger,
	}
}

// RegisterProcessor registers a processor and builds its handler with current shared components.
func (r *TaskProcessorRegistry) RegisterProcessor(platform string, processor worker.Processor) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.processors[platform] = processor
	r.handlers[platform] = r.newHandler(platform, processor)

	r.logger.Infof("registered task processor: platform=%s", platform)
}

// UpdateComponents refreshes shared components and rebuilds handlers in place.
func (r *TaskProcessorRegistry) UpdateComponents(
	resultReporter *ResultReporter,
	storeAPI api.StoreAPI,
	ownedStores []int64,
	useStoreQueues *bool,
	deduplicator *apptask.DeduplicationManager,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if resultReporter != nil {
		r.resultReporter = resultReporter
	}
	if storeAPI != nil {
		r.storeAPI = storeAPI
	}
	if ownedStores != nil {
		r.ownedStores = ownedStores
	}
	if useStoreQueues != nil {
		r.useStoreQueues = *useStoreQueues
	}
	if deduplicator != nil {
		r.deduplicator = deduplicator
	}

	for platform, processor := range r.processors {
		r.handlers[platform] = r.newHandler(platform, processor)
	}
}

// GetHandler returns the handler for a platform.
func (r *TaskProcessorRegistry) GetHandler(platform string) (rabbitmq.MessageHandler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handler, exists := r.handlers[platform]
	return handler, exists
}

// GetAllHandlers returns a copy of all handlers.
func (r *TaskProcessorRegistry) GetAllHandlers() map[string]rabbitmq.MessageHandler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := make(map[string]rabbitmq.MessageHandler, len(r.handlers))
	maps.Copy(handlers, r.handlers)
	return handlers
}

func (r *TaskProcessorRegistry) GetAllProcessors() map[string]worker.Processor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	processors := make(map[string]worker.Processor, len(r.processors))
	maps.Copy(processors, r.processors)
	return processors
}

// GetQueueName returns the queue name for a platform.
func (r *TaskProcessorRegistry) GetQueueName(platform string) string {
	return apptask.NewMessageAdapter().GetQueueName(platform)
}

// GetStats returns registry statistics.
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

func (r *TaskProcessorRegistry) newHandler(platform string, processor worker.Processor) rabbitmq.MessageHandler {
	return NewTaskHandler(TaskHandlerConfig{
		Platform:       platform,
		Processor:      processor,
		ResultReporter: r.resultReporter,
		StoreAPI:       r.storeAPI,
		OwnedStores:    r.ownedStores,
		UseStoreQueues: r.useStoreQueues,
		Deduplicator:   r.deduplicator,
		Logger:         r.logger,
	})
}

// Package rabbitmq 提供RabbitMQ任务处理器适配器
package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/internal/app/worker"

	"github.com/sirupsen/logrus"
)

// TaskHandler RabbitMQ任务处理器
type TaskHandler struct {
	processors map[string]worker.Processor // platform -> processor
	adapter    *TaskMessageAdapter
	logger     *logrus.Logger
}

// NewTaskHandler 创建任务处理器
func NewTaskHandler(logger *logrus.Logger) *TaskHandler {
	return &TaskHandler{
		processors: make(map[string]worker.Processor),
		adapter:    NewTaskMessageAdapter(),
		logger:     logger,
	}
}

// RegisterProcessor 注册平台处理器
func (th *TaskHandler) RegisterProcessor(platform string, processor worker.Processor) {
	th.processors[platform] = processor
	th.logger.Infof("注册平台处理器: %s", platform)
}

// HandleMessage 实现MessageHandler接口
func (th *TaskHandler) HandleMessage(ctx context.Context, msg *Message) error {
	startTime := time.Now()

	th.logger.Debugf("开始处理任务消息: ID=%s, Type=%s", msg.ID, msg.Type)

	// 将消息转换为任务对象
	task, err := th.adapter.MessageToTask(msg)
	if err != nil {
		return fmt.Errorf("转换消息为任务失败: %w", err)
	}

	th.logger.Infof("处理任务: ID=%d, Platform=%s, ProductID=%s, Priority=%d",
		task.ID, task.Platform, task.ProductID, task.Priority)

	// 获取对应平台的处理器
	processor, exists := th.processors[task.Platform]
	if !exists {
		return fmt.Errorf("未找到平台 %s 的处理器", task.Platform)
	}

	// 处理任务
	err = processor.ProcessTask(ctx, task)
	if err != nil {
		th.logger.Errorf("任务处理失败: ID=%d, Platform=%s, Error=%v",
			task.ID, task.Platform, err)
		return fmt.Errorf("处理任务失败: %w", err)
	}

	processingTime := time.Since(startTime)
	th.logger.Infof("任务处理成功: ID=%d, Platform=%s, Duration=%v",
		task.ID, task.Platform, processingTime)

	return nil
}

// GetProcessorStats 获取处理器统计信息
func (th *TaskHandler) GetProcessorStats() map[string]interface{} {
	stats := make(map[string]interface{})

	for platform := range th.processors {
		stats[platform] = map[string]interface{}{
			"status": "active",
		}
	}

	return stats
}

// PlatformTaskHandler 平台特定的任务处理器
type PlatformTaskHandler struct {
	platform  string
	processor worker.Processor
	adapter   *TaskMessageAdapter
	logger    *logrus.Logger
}

// NewPlatformTaskHandler 创建平台特定的任务处理器
func NewPlatformTaskHandler(platform string, processor worker.Processor, logger *logrus.Logger) *PlatformTaskHandler {
	return &PlatformTaskHandler{
		platform:  platform,
		processor: processor,
		adapter:   NewTaskMessageAdapter(),
		logger:    logger,
	}
}

// HandleMessage 处理平台特定的消息
func (pth *PlatformTaskHandler) HandleMessage(ctx context.Context, msg *Message) error {
	startTime := time.Now()

	pth.logger.Debugf("[%s] 开始处理任务消息: ID=%s", pth.platform, msg.ID)

	// 将消息转换为任务对象
	task, err := pth.adapter.MessageToTask(msg)
	if err != nil {
		return fmt.Errorf("转换消息为任务失败: %w", err)
	}

	// 验证平台匹配
	if task.Platform != pth.platform {
		return fmt.Errorf("任务平台 %s 与处理器平台 %s 不匹配", task.Platform, pth.platform)
	}

	pth.logger.Infof("[%s] 处理任务: ID=%d, ProductID=%s, Priority=%d",
		pth.platform, task.ID, task.ProductID, task.Priority)

	// 处理任务
	err = pth.processor.ProcessTask(ctx, task)
	if err != nil {
		pth.logger.Errorf("[%s] 任务处理失败: ID=%d, Error=%v",
			pth.platform, task.ID, err)
		return fmt.Errorf("处理任务失败: %w", err)
	}

	processingTime := time.Since(startTime)
	pth.logger.Infof("[%s] 任务处理成功: ID=%d, Duration=%v",
		pth.platform, task.ID, processingTime)

	return nil
}

// TaskProcessorRegistry 任务处理器注册表
type TaskProcessorRegistry struct {
	processors map[string]worker.Processor
	handlers   map[string]MessageHandler
	logger     *logrus.Logger
	mutex      sync.RWMutex
}

// NewTaskProcessorRegistry 创建任务处理器注册表
func NewTaskProcessorRegistry(logger *logrus.Logger) *TaskProcessorRegistry {
	return &TaskProcessorRegistry{
		processors: make(map[string]worker.Processor),
		handlers:   make(map[string]MessageHandler),
		logger:     logger,
	}
}

// RegisterProcessor 注册处理器
func (tpr *TaskProcessorRegistry) RegisterProcessor(platform string, processor worker.Processor) {
	tpr.mutex.Lock()
	defer tpr.mutex.Unlock()

	tpr.processors[platform] = processor

	// 创建平台特定的消息处理器
	handler := NewPlatformTaskHandler(platform, processor, tpr.logger)
	tpr.handlers[platform] = handler

	tpr.logger.Infof("注册处理器: 平台=%s", platform)
}

// GetHandler 获取消息处理器
func (tpr *TaskProcessorRegistry) GetHandler(platform string) (MessageHandler, bool) {
	tpr.mutex.RLock()
	defer tpr.mutex.RUnlock()

	handler, exists := tpr.handlers[platform]
	return handler, exists
}

// GetAllHandlers 获取所有消息处理器
func (tpr *TaskProcessorRegistry) GetAllHandlers() map[string]MessageHandler {
	tpr.mutex.RLock()
	defer tpr.mutex.RUnlock()

	handlers := make(map[string]MessageHandler)
	for platform, handler := range tpr.handlers {
		handlers[platform] = handler
	}

	return handlers
}

// GetQueueName 根据平台获取队列名称
func (tpr *TaskProcessorRegistry) GetQueueName(platform string) string {
	adapter := NewTaskMessageAdapter()
	return adapter.GetQueueName(platform)
}

// GetStats 获取统计信息
func (tpr *TaskProcessorRegistry) GetStats() map[string]interface{} {
	tpr.mutex.RLock()
	defer tpr.mutex.RUnlock()

	stats := make(map[string]interface{})
	stats["total_processors"] = len(tpr.processors)

	platformStats := make(map[string]interface{})
	for platform := range tpr.processors {
		platformStats[platform] = map[string]interface{}{
			"status": "registered",
		}
	}
	stats["platforms"] = platformStats

	return stats
}

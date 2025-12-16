// Package dispatcher 提供核心任务分发功能
package dispatcher

import (
	"context"
	"fmt"
	"sync"
	"time"

	"task-processor/common/types"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// taskDispatcher 任务分发器实现
type taskDispatcher struct {
	registry         ProcessorRegistry
	router           TaskRouter
	metricsCollector MetricsCollector
	logger           *logrus.Logger

	// 状态管理
	isRunning bool
	startTime time.Time
	mutex     sync.RWMutex

	// 配置
	config *DispatcherConfig
}

// DispatcherConfig 分发器配置
type DispatcherConfig struct {
	MaxRetries     int           `json:"max_retries"`     // 最大重试次数
	RetryDelay     time.Duration `json:"retry_delay"`     // 重试延迟
	BatchSize      int           `json:"batch_size"`      // 批处理大小
	ProcessTimeout time.Duration `json:"process_timeout"` // 处理超时时间
	EnableMetrics  bool          `json:"enable_metrics"`  // 启用指标收集
	LogLevel       string        `json:"log_level"`       // 日志级别
}

// DefaultDispatcherConfig 默认配置
func DefaultDispatcherConfig() *DispatcherConfig {
	return &DispatcherConfig{
		MaxRetries:     3,
		RetryDelay:     5 * time.Second,
		BatchSize:      10,
		ProcessTimeout: 30 * time.Minute,
		EnableMetrics:  true,
		LogLevel:       "info",
	}
}

// NewTaskDispatcher 创建任务分发器
func NewTaskDispatcher(logger *logrus.Logger, config *DispatcherConfig) TaskDispatcher {
	if config == nil {
		config = DefaultDispatcherConfig()
	}

	dispatcher := &taskDispatcher{
		registry:         NewProcessorRegistry(logger),
		router:           NewTaskRouter(logger),
		metricsCollector: NewMetricsCollector(logger),
		logger:           logger,
		config:           config,
	}

	logger.Info("[Dispatcher] 任务分发器创建完成")
	return dispatcher
}

// RegisterProcessor 注册平台处理器
func (d *taskDispatcher) RegisterProcessor(processor PlatformProcessor) error {
	if processor == nil {
		return fmt.Errorf("处理器不能为空")
	}

	// 注册到注册表
	if err := d.registry.Register(processor); err != nil {
		return fmt.Errorf("注册处理器失败: %w", err)
	}

	d.logger.Infof("[Dispatcher] 成功注册处理器: %s", processor.GetPlatformName())
	return nil
}

// UnregisterProcessor 注销平台处理器
func (d *taskDispatcher) UnregisterProcessor(platform string) error {
	if err := d.registry.Unregister(platform); err != nil {
		return fmt.Errorf("注销处理器失败: %w", err)
	}

	d.logger.Infof("[Dispatcher] 成功注销处理器: %s", platform)
	return nil
}

// DispatchTask 分发任务
func (d *taskDispatcher) DispatchTask(ctx context.Context, task *model.UnifiedTask) error {
	if task == nil {
		return fmt.Errorf("任务不能为空")
	}

	d.mutex.RLock()
	if !d.isRunning {
		d.mutex.RUnlock()
		return fmt.Errorf("分发器未启动")
	}
	d.mutex.RUnlock()

	// 更新任务状态
	task.UpdateStatus(types.TaskStatusQueued, "任务已进入分发队列")

	// 路由任务
	targetPlatform, err := d.router.Route(task)
	if err != nil {
		task.UpdateStatus(types.TaskStatusCrawlFailed, fmt.Sprintf("路由失败: %v", err))
		return fmt.Errorf("路由任务失败: %w", err)
	}

	// 获取目标处理器
	processor, err := d.registry.Get(targetPlatform)
	if err != nil {
		task.UpdateStatus(types.TaskStatusCrawlFailed, fmt.Sprintf("获取处理器失败: %v", err))
		return fmt.Errorf("获取处理器失败: %w", err)
	}

	// 检查处理器是否可以处理任务
	if !processor.CanHandle(task) {
		task.UpdateStatus(types.TaskStatusCrawlFailed, "处理器无法处理此任务")
		return fmt.Errorf("处理器 %s 无法处理任务 %s", targetPlatform, task.ID)
	}

	// 执行任务处理
	return d.executeTask(ctx, processor, task)
}

// executeTask 执行任务处理
func (d *taskDispatcher) executeTask(ctx context.Context, processor PlatformProcessor, task *model.UnifiedTask) error {
	platform := processor.GetPlatformName()

	// 创建带超时的上下文
	taskCtx, cancel := context.WithTimeout(ctx, d.config.ProcessTimeout)
	defer cancel()

	// 记录任务开始
	if d.config.EnableMetrics {
		d.metricsCollector.RecordTaskStart(platform, task.ID)
	}

	// 更新任务状态
	task.UpdateStatus(types.TaskStatusProcessing, fmt.Sprintf("开始在平台 %s 上处理", platform))

	d.logger.Infof("[Dispatcher] 开始处理任务: ID=%s, Platform=%s", task.ID, platform)

	// 执行任务处理
	startTime := time.Now()
	err := processor.ProcessTask(taskCtx, task)
	duration := time.Since(startTime)

	// 记录结果
	if err != nil {
		task.UpdateStatus(types.TaskStatusCrawlFailed, fmt.Sprintf("处理失败: %v", err))
		if d.config.EnableMetrics {
			d.metricsCollector.RecordTaskFailure(platform, task.ID, duration, err)
		}

		d.logger.Errorf("[Dispatcher] 任务处理失败: ID=%s, Platform=%s, Error=%v", task.ID, platform, err)

		// 检查是否可以重试
		if task.CanRetry(d.config.MaxRetries) {
			return d.retryTask(ctx, processor, task)
		}

		return fmt.Errorf("任务处理失败: %w", err)
	}

	// 处理成功
	task.UpdateStatus(types.TaskStatusPublished, "任务处理完成")
	if d.config.EnableMetrics {
		d.metricsCollector.RecordTaskSuccess(platform, task.ID, duration)
	}

	d.logger.Infof("[Dispatcher] 任务处理成功: ID=%s, Platform=%s, Duration=%v", task.ID, platform, duration)
	return nil
}

// retryTask 重试任务
func (d *taskDispatcher) retryTask(ctx context.Context, processor PlatformProcessor, task *model.UnifiedTask) error {
	task.RetryCount++
	task.UpdateStatus(types.TaskStatusPendingRetry, fmt.Sprintf("准备第 %d 次重试", task.RetryCount))

	d.logger.Warnf("[Dispatcher] 任务重试: ID=%s, RetryCount=%d", task.ID, task.RetryCount)

	// 等待重试延迟
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d.config.RetryDelay):
		// 继续重试
	}

	return d.executeTask(ctx, processor, task)
}

// DispatchBatch 批量分发任务
func (d *taskDispatcher) DispatchBatch(ctx context.Context, tasks []*model.UnifiedTask) error {
	if len(tasks) == 0 {
		return nil
	}

	d.logger.Infof("[Dispatcher] 开始批量处理任务: 数量=%d", len(tasks))

	// 按批次大小分组处理
	batchSize := d.config.BatchSize
	if batchSize <= 0 {
		batchSize = 10
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(tasks))

	for i := 0; i < len(tasks); i += batchSize {
		end := i + batchSize
		if end > len(tasks) {
			end = len(tasks)
		}

		batch := tasks[i:end]
		wg.Add(1)

		go func(batch []*model.UnifiedTask) {
			defer wg.Done()

			for _, task := range batch {
				if err := d.DispatchTask(ctx, task); err != nil {
					errChan <- fmt.Errorf("任务 %s 处理失败: %w", task.ID, err)
				}
			}
		}(batch)
	}

	wg.Wait()
	close(errChan)

	// 收集错误
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		d.logger.Errorf("[Dispatcher] 批量处理完成，失败数量: %d", len(errors))
		return fmt.Errorf("批量处理中有 %d 个任务失败", len(errors))
	}

	d.logger.Infof("[Dispatcher] 批量处理完成，成功数量: %d", len(tasks))
	return nil
}

// Start 启动分发器
func (d *taskDispatcher) Start(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.isRunning {
		return fmt.Errorf("分发器已经启动")
	}

	d.isRunning = true
	d.startTime = time.Now()

	// 启动所有注册的处理器
	processors := d.registry.GetAll()
	for platform, processor := range processors {
		if err := processor.Start(ctx); err != nil {
			d.logger.Errorf("[Dispatcher] 启动处理器失败: %s, Error=%v", platform, err)
		} else {
			d.logger.Infof("[Dispatcher] 启动处理器成功: %s", platform)
		}
	}

	d.logger.Info("[Dispatcher] 任务分发器启动完成")
	return nil
}

// Stop 停止分发器
func (d *taskDispatcher) Stop(ctx context.Context) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if !d.isRunning {
		return fmt.Errorf("分发器未启动")
	}

	// 停止所有处理器
	processors := d.registry.GetAll()
	for platform, processor := range processors {
		if err := processor.Stop(ctx); err != nil {
			d.logger.Errorf("[Dispatcher] 停止处理器失败: %s, Error=%v", platform, err)
		} else {
			d.logger.Infof("[Dispatcher] 停止处理器成功: %s", platform)
		}
	}

	d.isRunning = false
	d.logger.Info("[Dispatcher] 任务分发器停止完成")
	return nil
}

// GetProcessorStatus 获取处理器状态
func (d *taskDispatcher) GetProcessorStatus(platform string) (*ProcessorStatus, error) {
	processor, err := d.registry.Get(platform)
	if err != nil {
		return nil, err
	}

	return processor.GetStatus(), nil
}

// GetAllProcessorStatus 获取所有处理器状态
func (d *taskDispatcher) GetAllProcessorStatus() map[string]*ProcessorStatus {
	processors := d.registry.GetAll()
	statuses := make(map[string]*ProcessorStatus)

	for platform, processor := range processors {
		statuses[platform] = processor.GetStatus()
	}

	return statuses
}

// GetSupportedPlatforms 获取支持的平台列表
func (d *taskDispatcher) GetSupportedPlatforms() []string {
	return d.registry.List()
}

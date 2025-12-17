// Package service 提供任务服务层功能
package service

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/dispatcher"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// TaskService 任务服务接口
type TaskService interface {
	// SubmitTask 提交单个任务
	SubmitTask(ctx context.Context, task *model.UnifiedTask) error

	// SubmitBatch 批量提交任务
	SubmitBatch(ctx context.Context, tasks []*model.UnifiedTask) error

	// GetTaskStatus 获取任务状态
	GetTaskStatus(taskID string) (*model.UnifiedTask, error)

	// CancelTask 取消任务
	CancelTask(taskID string) error

	// RetryTask 重试任务
	RetryTask(ctx context.Context, taskID string) error

	// GetPlatformStatus 获取平台状态
	GetPlatformStatus() map[string]*dispatcher.ProcessorStatus

	// GetMetrics 获取任务指标
	GetMetrics() map[string]*dispatcher.TaskMetrics
}

// taskService 任务服务实现
type taskService struct {
	dispatcher dispatcher.TaskDispatcher
	taskStore  TaskStore
	logger     *logrus.Logger
	config     *TaskServiceConfig
}

// TaskServiceConfig 任务服务配置
type TaskServiceConfig struct {
	MaxRetries        int           `json:"max_retries"`
	DefaultTimeout    time.Duration `json:"default_timeout"`
	BatchSize         int           `json:"batch_size"`
	EnablePersistence bool          `json:"enable_persistence"`
}

// DefaultTaskServiceConfig 默认配置
func DefaultTaskServiceConfig() *TaskServiceConfig {
	return &TaskServiceConfig{
		MaxRetries:        3,
		DefaultTimeout:    30 * time.Minute,
		BatchSize:         50,
		EnablePersistence: true,
	}
}

// TaskStore 任务存储接口
type TaskStore interface {
	// Save 保存任务
	Save(task *model.UnifiedTask) error

	// Get 获取任务
	Get(taskID string) (*model.UnifiedTask, error)

	// Update 更新任务
	Update(task *model.UnifiedTask) error

	// Delete 删除任务
	Delete(taskID string) error

	// List 列出任务
	List(filter TaskFilter) ([]*model.UnifiedTask, error)
}

// TaskFilter 任务过滤器
type TaskFilter struct {
	Platform  string           `json:"platform"`
	Status    model.TaskStatus `json:"status"`
	StoreID   int64            `json:"store_id"`
	TenantID  int64            `json:"tenant_id"`
	StartTime *time.Time       `json:"start_time"`
	EndTime   *time.Time       `json:"end_time"`
	Limit     int              `json:"limit"`
	Offset    int              `json:"offset"`
}

// NewTaskService 创建任务服务
func NewTaskService(
	dispatcher dispatcher.TaskDispatcher,
	taskStore TaskStore,
	logger *logrus.Logger,
	config *TaskServiceConfig,
) TaskService {
	if config == nil {
		config = DefaultTaskServiceConfig()
	}

	return &taskService{
		dispatcher: dispatcher,
		taskStore:  taskStore,
		logger:     logger,
		config:     config,
	}
}

// SubmitTask 提交单个任务
func (s *taskService) SubmitTask(ctx context.Context, task *model.UnifiedTask) error {
	if task == nil {
		return fmt.Errorf("任务不能为空")
	}

	s.logger.Infof("[TaskService] 提交任务: ID=%s, Platform=%s", task.ID, task.TargetPlatform)

	// 验证任务
	if err := s.validateTask(task); err != nil {
		return fmt.Errorf("任务验证失败: %w", err)
	}

	// 设置默认值
	s.setTaskDefaults(task)

	// 持久化任务（如果启用）
	if s.config.EnablePersistence && s.taskStore != nil {
		if err := s.taskStore.Save(task); err != nil {
			s.logger.Errorf("[TaskService] 保存任务失败: %v", err)
			// 不阻塞任务提交，只记录错误
		}
	}

	// 提交到分发器
	if err := s.dispatcher.DispatchTask(ctx, task); err != nil {
		// 更新任务状态
		task.UpdateStatus(model.TaskStatusCrawlFailed, fmt.Sprintf("分发失败: %v", err))
		s.updateTaskInStore(task)
		return fmt.Errorf("任务分发失败: %w", err)
	}

	s.logger.Infof("[TaskService] 任务提交成功: ID=%s", task.ID)
	return nil
}

// SubmitBatch 批量提交任务
func (s *taskService) SubmitBatch(ctx context.Context, tasks []*model.UnifiedTask) error {
	if len(tasks) == 0 {
		return nil
	}

	s.logger.Infof("[TaskService] 批量提交任务: 数量=%d", len(tasks))

	// 验证所有任务
	for i, task := range tasks {
		if err := s.validateTask(task); err != nil {
			return fmt.Errorf("任务 %d 验证失败: %w", i, err)
		}
		s.setTaskDefaults(task)
	}

	// 批量持久化（如果启用）
	if s.config.EnablePersistence && s.taskStore != nil {
		for _, task := range tasks {
			if err := s.taskStore.Save(task); err != nil {
				s.logger.Errorf("[TaskService] 保存任务失败: ID=%s, Error=%v", task.ID, err)
			}
		}
	}

	// 批量提交到分发器
	if err := s.dispatcher.DispatchBatch(ctx, tasks); err != nil {
		s.logger.Errorf("[TaskService] 批量任务分发失败: %v", err)
		return fmt.Errorf("批量任务分发失败: %w", err)
	}

	s.logger.Infof("[TaskService] 批量任务提交成功: 数量=%d", len(tasks))
	return nil
}

// GetTaskStatus 获取任务状态
func (s *taskService) GetTaskStatus(taskID string) (*model.UnifiedTask, error) {
	if taskID == "" {
		return nil, fmt.Errorf("任务ID不能为空")
	}

	if s.taskStore == nil {
		return nil, fmt.Errorf("任务存储未配置")
	}

	task, err := s.taskStore.Get(taskID)
	if err != nil {
		return nil, fmt.Errorf("获取任务状态失败: %w", err)
	}

	return task, nil
}

// CancelTask 取消任务
func (s *taskService) CancelTask(taskID string) error {
	if taskID == "" {
		return fmt.Errorf("任务ID不能为空")
	}

	s.logger.Infof("[TaskService] 取消任务: ID=%s", taskID)

	// 获取任务
	task, err := s.GetTaskStatus(taskID)
	if err != nil {
		return fmt.Errorf("获取任务失败: %w", err)
	}

	// 检查任务状态
	if task.IsCompleted() {
		return fmt.Errorf("任务已完成，无法取消")
	}

	// 更新任务状态
	task.UpdateStatus(model.TaskStatusCancelled, "任务已被用户取消")

	// 更新存储
	if err := s.updateTaskInStore(task); err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	s.logger.Infof("[TaskService] 任务取消成功: ID=%s", taskID)
	return nil
}

// RetryTask 重试任务
func (s *taskService) RetryTask(ctx context.Context, taskID string) error {
	if taskID == "" {
		return fmt.Errorf("任务ID不能为空")
	}

	s.logger.Infof("[TaskService] 重试任务: ID=%s", taskID)

	// 获取任务
	task, err := s.GetTaskStatus(taskID)
	if err != nil {
		return fmt.Errorf("获取任务失败: %w", err)
	}

	// 检查是否可以重试
	if !task.CanRetry(s.config.MaxRetries) {
		return fmt.Errorf("任务不能重试: 重试次数已达上限或状态不允许")
	}

	// 重置任务状态
	task.RetryCount++
	task.UpdateStatus(model.TaskStatusPending, "任务准备重试")

	// 更新存储
	if err := s.updateTaskInStore(task); err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	// 重新提交任务
	if err := s.dispatcher.DispatchTask(ctx, task); err != nil {
		return fmt.Errorf("重新分发任务失败: %w", err)
	}

	s.logger.Infof("[TaskService] 任务重试成功: ID=%s, RetryCount=%d", taskID, task.RetryCount)
	return nil
}

// GetPlatformStatus 获取平台状态
func (s *taskService) GetPlatformStatus() map[string]*dispatcher.ProcessorStatus {
	return s.dispatcher.GetAllProcessorStatus()
}

// GetMetrics 获取任务指标
func (s *taskService) GetMetrics() map[string]*dispatcher.TaskMetrics {
	// 这里需要从分发器获取指标收集器
	// 简化实现，返回空指标
	return make(map[string]*dispatcher.TaskMetrics)
}

// validateTask 验证任务
func (s *taskService) validateTask(task *model.UnifiedTask) error {
	if task.ID == "" {
		return fmt.Errorf("任务ID不能为空")
	}
	if task.ProductID == "" {
		return fmt.Errorf("产品ID不能为空")
	}
	if task.TargetPlatform == "" {
		return fmt.Errorf("目标平台不能为空")
	}
	if task.StoreID == 0 {
		return fmt.Errorf("店铺ID不能为空")
	}
	if task.TenantID == 0 {
		return fmt.Errorf("租户ID不能为空")
	}

	return nil
}

// setTaskDefaults 设置任务默认值
func (s *taskService) setTaskDefaults(task *model.UnifiedTask) {
	now := time.Now()

	if task.CreateTime == 0 {
		task.CreateTime = now.Unix()
	}
	if task.Status == 0 {
		task.Status = model.TaskStatusPending
	}
	if task.Priority == 0 {
		task.Priority = 5 // 默认优先级
	}
	if task.Metadata == nil {
		task.Metadata = make(map[string]interface{})
	}
	if task.ProcessingConfig == nil {
		task.ProcessingConfig = make(map[string]interface{})
	}

	// 设置平台特定默认值
	switch task.TargetPlatform {
	case "amazon":
		if task.MarketplaceID == "" {
			task.MarketplaceID = "ATVPDKIKX0DER" // 美国市场
		}
		if task.LanguageTag == "" {
			task.LanguageTag = "en_US"
		}
		if task.Currency == "" {
			task.Currency = "USD"
		}
	case "temu", "shein":
		if task.Region == "" {
			task.Region = "US"
		}
	}
}

// updateTaskInStore 更新任务存储
func (s *taskService) updateTaskInStore(task *model.UnifiedTask) error {
	if s.config.EnablePersistence && s.taskStore != nil {
		return s.taskStore.Update(task)
	}
	return nil
}

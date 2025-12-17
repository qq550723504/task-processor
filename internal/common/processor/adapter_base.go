// Package processor 提供处理器适配器基础实现
package processor

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/dispatcher"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

// BaseProcessorAdapter 基础处理器适配器
// 包含所有平台适配器的通用逻辑
type BaseProcessorAdapter struct {
	status   *dispatcher.ProcessorStatus
	logger   *logrus.Logger
	platform string
}

// NewBaseProcessorAdapter 创建基础适配器
func NewBaseProcessorAdapter(platform string, logger *logrus.Logger) *BaseProcessorAdapter {
	return &BaseProcessorAdapter{
		platform: platform,
		logger:   logger,
		status: &dispatcher.ProcessorStatus{
			Name:           fmt.Sprintf("%s处理器适配器", platform),
			Platform:       platform,
			Status:         "stopped",
			TasksProcessed: 0,
			TasksSucceeded: 0,
			TasksFailed:    0,
			AvailableSlots: 10, // 默认可用槽位
			Metrics:        make(map[string]any),
		},
	}
}

// StartBase 基础启动逻辑
func (b *BaseProcessorAdapter) StartBase(ctx context.Context) {
	b.logger.Infof("[%sAdapter] 启动%s处理器适配器", b.platform, b.platform)

	// 更新状态
	b.status.Status = "running"
	b.status.StartTime = time.Now()
	b.status.LastActiveTime = time.Now()
	b.status.ErrorMessage = ""
}

// StopBase 基础停止逻辑
func (b *BaseProcessorAdapter) StopBase(ctx context.Context) {
	b.logger.Infof("[%sAdapter] 停止%s处理器适配器", b.platform, b.platform)

	// 更新状态
	b.status.Status = "stopped"
	b.status.ErrorMessage = ""
}

// ProcessTaskBase 基础任务处理逻辑
func (b *BaseProcessorAdapter) ProcessTaskBase(task *model.UnifiedTask) error {
	if task == nil {
		return fmt.Errorf("任务不能为空")
	}

	b.logger.Infof("[%sAdapter] 开始处理任务: ID=%s", b.platform, task.ID)

	// 更新状态
	b.status.TasksProcessed++
	b.status.LastActiveTime = time.Now()

	return nil
}

// OnTaskSuccess 任务成功处理
func (b *BaseProcessorAdapter) OnTaskSuccess(taskID string) {
	b.status.TasksSucceeded++
	b.status.ErrorMessage = ""
	b.logger.Infof("[%sAdapter] 任务处理完成: ID=%s", b.platform, taskID)
}

// OnTaskFailure 任务处理失败
func (b *BaseProcessorAdapter) OnTaskFailure(taskID string, err error) {
	b.status.TasksFailed++
	b.status.ErrorMessage = err.Error()
	b.logger.Errorf("[%sAdapter] 任务处理失败: ID=%s, Error=%v", b.platform, taskID, err)
}

// GetStatusBase 获取基础状态
func (b *BaseProcessorAdapter) GetStatusBase() *dispatcher.ProcessorStatus {
	// 更新指标
	b.status.Metrics = map[string]any{
		"success_rate": b.calculateSuccessRate(),
		"uptime":       time.Since(b.status.StartTime).String(),
	}

	// 返回状态副本
	return &dispatcher.ProcessorStatus{
		Name:           b.status.Name,
		Platform:       b.status.Platform,
		Status:         b.status.Status,
		StartTime:      b.status.StartTime,
		LastActiveTime: b.status.LastActiveTime,
		TasksProcessed: b.status.TasksProcessed,
		TasksSucceeded: b.status.TasksSucceeded,
		TasksFailed:    b.status.TasksFailed,
		AvailableSlots: b.status.AvailableSlots,
		Metrics:        b.status.Metrics,
		ErrorMessage:   b.status.ErrorMessage,
	}
}

// GetPlatformName 获取平台名称
func (b *BaseProcessorAdapter) GetPlatformName() string {
	return b.platform
}

// CanHandleBase 基础任务处理能力检查
func (b *BaseProcessorAdapter) CanHandleBase(task *model.UnifiedTask, targetPlatform string) bool {
	if task == nil {
		return false
	}

	// 检查目标平台
	if task.TargetPlatform != targetPlatform {
		return false
	}

	// 检查必要字段
	if task.ProductID == "" || task.StoreID == 0 {
		b.logger.Warnf("[%sAdapter] 任务缺少必要字段: ProductID=%s, StoreID=%d",
			b.platform, task.ProductID, task.StoreID)
		return false
	}

	// 检查处理器状态
	if b.status.Status != "running" {
		b.logger.Warnf("[%sAdapter] 处理器未运行，状态: %s", b.platform, b.status.Status)
		return false
	}

	// 检查可用槽位
	if b.status.AvailableSlots <= 0 {
		b.logger.Warnf("[%sAdapter] 没有可用槽位", b.platform)
		return false
	}

	return true
}

// UpdateAvailableSlots 更新可用槽位
func (b *BaseProcessorAdapter) UpdateAvailableSlots(slots int) {
	b.status.AvailableSlots = slots
}

// calculateSuccessRate 计算成功率
func (b *BaseProcessorAdapter) calculateSuccessRate() float64 {
	if b.status.TasksProcessed == 0 {
		return 0.0
	}
	return float64(b.status.TasksSucceeded) / float64(b.status.TasksProcessed) * 100.0
}

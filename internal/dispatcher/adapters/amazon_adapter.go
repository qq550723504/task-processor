// Package adapters 提供平台适配器实现
package adapters

import (
	"context"
	"fmt"
	"time"

	"task-processor/internal/dispatcher"
	"task-processor/internal/model"
	"task-processor/internal/platforms/amazon"

	"github.com/sirupsen/logrus"
)

// AmazonProcessorAdapter Amazon平台处理器适配器
type AmazonProcessorAdapter struct {
	processor *amazon.Processor
	logger    *logrus.Logger
	status    *dispatcher.ProcessorStatus
}

// NewAmazonProcessorAdapter 创建Amazon处理器适配器
func NewAmazonProcessorAdapter(processor *amazon.Processor, logger *logrus.Logger) dispatcher.PlatformProcessor {
	return &AmazonProcessorAdapter{
		processor: processor,
		logger:    logger,
		status: &dispatcher.ProcessorStatus{
			Name:           "Amazon处理器适配器",
			Platform:       "amazon",
			Status:         "stopped",
			TasksProcessed: 0,
			TasksSucceeded: 0,
			TasksFailed:    0,
			AvailableSlots: 10, // 默认可用槽位
			Metrics:        make(map[string]interface{}),
		},
	}
}

// ProcessTask 处理任务
func (a *AmazonProcessorAdapter) ProcessTask(ctx context.Context, task *model.UnifiedTask) error {
	if task == nil {
		return fmt.Errorf("任务不能为空")
	}

	a.logger.Infof("[AmazonAdapter] 开始处理任务: ID=%s", task.ID)

	// 更新状态
	a.status.TasksProcessed++
	a.status.LastActiveTime = time.Now()

	// 转换为Amazon处理器需要的数据格式
	taskData, err := a.convertToTaskData(task)
	if err != nil {
		a.status.TasksFailed++
		return fmt.Errorf("转换任务数据失败: %w", err)
	}

	// 调用Amazon处理器的管道处理方法
	err = a.processor.ProcessTaskWithPipeline(ctx, taskData)
	if err != nil {
		a.status.TasksFailed++
		a.status.ErrorMessage = err.Error()
		return fmt.Errorf("Amazon处理器执行失败: %w", err)
	}

	// 处理成功
	a.status.TasksSucceeded++
	a.status.ErrorMessage = ""

	a.logger.Infof("[AmazonAdapter] 任务处理完成: ID=%s", task.ID)
	return nil
}

// convertToTaskData 将UnifiedTask转换为Amazon处理器需要的数据格式
func (a *AmazonProcessorAdapter) convertToTaskData(task *model.UnifiedTask) (map[string]interface{}, error) {
	// 转换任务数据
	taskData := task.ToTaskData()

	// 创建Amazon处理器需要的数据格式
	amazonTaskData := map[string]interface{}{
		"product_id":        taskData.ProductID,
		"store_id":          taskData.StoreID,
		"tenant_id":         taskData.TenantID,
		"raw_json_data":     taskData.RawJSONData,
		"source_platform":   taskData.SourcePlatform,
		"target_platform":   taskData.TargetPlatform,
		"marketplace_id":    taskData.MarketplaceID,
		"language_tag":      taskData.LanguageTag,
		"currency_target":   taskData.Currency,
		"processing_config": taskData.ProcessingConfig,
		"metadata":          taskData.Metadata,
	}

	// 设置默认值
	if amazonTaskData["marketplace_id"] == "" {
		amazonTaskData["marketplace_id"] = "ATVPDKIKX0DER" // 美国市场
	}
	if amazonTaskData["language_tag"] == "" {
		amazonTaskData["language_tag"] = "en_US"
	}
	if amazonTaskData["currency_target"] == "" {
		amazonTaskData["currency_target"] = "USD"
	}

	a.logger.Debugf("[AmazonAdapter] 任务数据转换完成: TaskID=%s, MarketplaceID=%s",
		task.ID, amazonTaskData["marketplace_id"])

	return amazonTaskData, nil
}

// Start 启动处理器
func (a *AmazonProcessorAdapter) Start(ctx context.Context) error {
	a.logger.Info("[AmazonAdapter] 启动Amazon处理器适配器")

	// 启动底层Amazon处理器
	if err := a.processor.Start(ctx); err != nil {
		a.status.Status = "error"
		a.status.ErrorMessage = err.Error()
		return fmt.Errorf("启动Amazon处理器失败: %w", err)
	}

	// 更新状态
	a.status.Status = "running"
	a.status.StartTime = time.Now()
	a.status.LastActiveTime = time.Now()
	a.status.ErrorMessage = ""

	a.logger.Info("[AmazonAdapter] Amazon处理器适配器启动完成")
	return nil
}

// Stop 停止处理器
func (a *AmazonProcessorAdapter) Stop(ctx context.Context) error {
	a.logger.Info("[AmazonAdapter] 停止Amazon处理器适配器")

	// 停止底层Amazon处理器
	a.processor.Close()

	// 更新状态
	a.status.Status = "stopped"
	a.status.ErrorMessage = ""

	a.logger.Info("[AmazonAdapter] Amazon处理器适配器停止完成")
	return nil
}

// GetStatus 获取处理器状态
func (a *AmazonProcessorAdapter) GetStatus() *dispatcher.ProcessorStatus {
	// 更新指标
	a.status.Metrics = map[string]interface{}{
		"success_rate": a.calculateSuccessRate(),
		"uptime":       time.Since(a.status.StartTime).String(),
	}

	// 返回状态副本
	return &dispatcher.ProcessorStatus{
		Name:           a.status.Name,
		Platform:       a.status.Platform,
		Status:         a.status.Status,
		StartTime:      a.status.StartTime,
		LastActiveTime: a.status.LastActiveTime,
		TasksProcessed: a.status.TasksProcessed,
		TasksSucceeded: a.status.TasksSucceeded,
		TasksFailed:    a.status.TasksFailed,
		AvailableSlots: a.status.AvailableSlots,
		Metrics:        a.status.Metrics,
		ErrorMessage:   a.status.ErrorMessage,
	}
}

// GetPlatformName 获取平台名称
func (a *AmazonProcessorAdapter) GetPlatformName() string {
	return "amazon"
}

// CanHandle 检查是否可以处理指定任务
func (a *AmazonProcessorAdapter) CanHandle(task *model.UnifiedTask) bool {
	if task == nil {
		return false
	}

	// 检查目标平台
	if task.TargetPlatform != "amazon" {
		return false
	}

	// 检查必要字段
	if task.ProductID == "" || task.RawJSONData == "" {
		a.logger.Warnf("[AmazonAdapter] 任务缺少必要字段: ProductID=%s, RawJSONData长度=%d",
			task.ProductID, len(task.RawJSONData))
		return false
	}

	// 检查处理器状态
	if a.status.Status != "running" {
		a.logger.Warnf("[AmazonAdapter] 处理器未运行，状态: %s", a.status.Status)
		return false
	}

	// 检查可用槽位
	if a.status.AvailableSlots <= 0 {
		a.logger.Warnf("[AmazonAdapter] 没有可用槽位")
		return false
	}

	return true
}

// calculateSuccessRate 计算成功率
func (a *AmazonProcessorAdapter) calculateSuccessRate() float64 {
	if a.status.TasksProcessed == 0 {
		return 0.0
	}
	return float64(a.status.TasksSucceeded) / float64(a.status.TasksProcessed) * 100.0
}

// GetProcessor 获取底层Amazon处理器（用于测试或特殊需求）
func (a *AmazonProcessorAdapter) GetProcessor() *amazon.Processor {
	return a.processor
}

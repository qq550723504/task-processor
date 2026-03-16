// Package task 提供任务提交器适配器
package task

import (
	"context"
	"encoding/json"
	"fmt"
	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// PlatformProcessor 平台处理器接口（现有处理器需要实现的最小接口）
type PlatformProcessor interface {
	// GetWorkerPool 获取工作池
	GetWorkerPool() worker.WorkerPool
	// GetPlatform 获取平台名称
	GetPlatform() string
}

// TaskSubmitterAdapter 任务提交器适配器
type TaskSubmitterAdapter struct {
	processor PlatformProcessor
	platform  string
	logger    *logrus.Logger
}

// NewTaskSubmitterAdapter 创建任务提交器适配器
func NewTaskSubmitterAdapter(processor PlatformProcessor, platform string, logger *logrus.Logger) TaskSubmitterAdapter {
	return TaskSubmitterAdapter{
		processor: processor,
		platform:  platform,
		logger:    logger,
	}
}

// SubmitTask 提交任务到对应平台的工作池
func (a TaskSubmitterAdapter) SubmitTask(ctx context.Context, taskData string) error {
	workerPool := a.processor.GetWorkerPool()
	if workerPool == nil {
		return fmt.Errorf("工作池未初始化")
	}

	// 从 taskData JSON 中解析任务信息
	var task struct {
		TenantID int64 `json:"tenantId"`
		StoreID  int64 `json:"storeId"`
	}

	if err := json.Unmarshal([]byte(taskData), &task); err != nil {
		a.logger.Errorf("解析任务数据失败: %v", err)
		// 如果解析失败，使用默认值（向后兼容）
		task.TenantID = 0
		task.StoreID = 0
	}

	// 创建WorkerJob并提交
	job := worker.WorkerJob{
		TaskData: taskData,
		TenantID: fmt.Sprintf("%d", task.TenantID),
		ShopID:   fmt.Sprintf("%d", task.StoreID),
	}

	return workerPool.Submit(job)
}

// GetPlatform 获取平台类型
func (a TaskSubmitterAdapter) GetPlatform() string {
	return a.platform
}

// GetAvailableSlots 获取可用槽位数
func (a TaskSubmitterAdapter) GetAvailableSlots() int {
	workerPool := a.processor.GetWorkerPool()
	if workerPool == nil {
		return 0
	}
	return workerPool.AvailableSlots()
}

// GetQueueStats 获取队列统计信息
func (a TaskSubmitterAdapter) GetQueueStats() worker.QueueStats {
	workerPool := a.processor.GetWorkerPool()
	if workerPool == nil {
		return worker.QueueStats{}
	}
	return workerPool.GetQueueStats()
}

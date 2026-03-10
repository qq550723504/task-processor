package pipeline

import (
	"context"
	"fmt"
	"task-processor/internal/infra/worker"
	"task-processor/internal/pkg/jsonutil"

	"github.com/sirupsen/logrus"
)

// SheinTaskSubmitter SHEIN任务提交器（适配器）
type SheinTaskSubmitter struct {
	workerPool worker.WorkerPool
}

// NewSheinTaskSubmitter 创建SHEIN任务提交器
func NewSheinTaskSubmitter(workerPool worker.WorkerPool) *SheinTaskSubmitter {
	return &SheinTaskSubmitter{
		workerPool: workerPool,
	}
}

// SubmitTask 提交任务
func (s *SheinTaskSubmitter) SubmitTask(ctx context.Context, taskData string) error {
	if s.workerPool == nil {
		return nil // SHEIN的workerPool可能未初始化
	}

	// 解析任务数据以获取必要的字段
	var task struct {
		TenantID int64 `json:"tenantId"`
		StoreID  int64 `json:"storeId"`
	}
	if err := jsonutil.UnmarshalString(taskData, &task, ""); err != nil {
		return err
	}

	job := worker.WorkerJob{
		TenantID: fmt.Sprintf("%d", task.TenantID),
		ShopID:   fmt.Sprintf("%d", task.StoreID),
		TaskData: taskData,
	}

	if err := s.workerPool.Submit(job); err != nil {
		return err
	}

	logrus.Infof("[SHEIN] ✅ 任务已提交到工作池")
	return nil
}

// GetPlatform 返回平台类型
func (s *SheinTaskSubmitter) GetPlatform() string {
	return "SHEIN"
}

// GetAvailableSlots 获取可用槽位数
func (s *SheinTaskSubmitter) GetAvailableSlots() int {
	if s.workerPool == nil {
		return 0
	}
	return s.workerPool.AvailableSlots()
}

// GetQueueStats 获取队列统计信息
func (s *SheinTaskSubmitter) GetQueueStats() worker.QueueStats {
	if s.workerPool == nil {
		return worker.QueueStats{}
	}
	return s.workerPool.GetQueueStats()
}

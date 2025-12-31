package temu

import (
	"context"
	"encoding/json"
	"fmt"
	"task-processor/internal/app/worker"
	"task-processor/internal/domain/model"
	"time"

	"github.com/sirupsen/logrus"
)

// TemuTaskSubmitter TEMU任务提交器（适配器）
type TemuTaskSubmitter struct {
	workerPool worker.WorkerPool
}

// NewTemuTaskSubmitter 创建TEMU任务提交器
func NewTemuTaskSubmitter(workerPool worker.WorkerPool) *TemuTaskSubmitter {
	return &TemuTaskSubmitter{
		workerPool: workerPool,
	}
}

// SubmitTask 提交任务
func (s *TemuTaskSubmitter) SubmitTask(ctx context.Context, taskData string) error {
	var task model.Task
	if err := json.Unmarshal([]byte(taskData), &task); err != nil {
		return fmt.Errorf("解析任务数据失败: %w", err)
	}

	// 记录等待时间
	if task.CreateTime > 0 {
		waitTime := time.Since(time.Unix(task.CreateTime/1000, 0))
		logrus.Infof("[TEMU] Task %d: Pending -> Processing (Priority: %d, WaitTime: %v)",
			task.ID, task.Priority, waitTime.Truncate(time.Millisecond))
	}

	// 提交到工作池
	if err := s.workerPool.Submit(worker.WorkerJob{
		TenantID: fmt.Sprintf("%d", task.TenantID),
		ShopID:   fmt.Sprintf("%d", task.StoreID),
		TaskData: taskData,
	}); err != nil {
		return fmt.Errorf("提交任务到工作池失败: %w", err)
	}

	logrus.Infof("[TEMU] ✅ 任务已提交到工作池: ID=%d, ProductID=%s", task.ID, task.ProductID)
	return nil
}

// GetPlatform 返回平台类型
func (s *TemuTaskSubmitter) GetPlatform() string {
	return "TEMU"
}

// GetAvailableSlots 获取可用槽位数
func (s *TemuTaskSubmitter) GetAvailableSlots() int {
	if s.workerPool == nil {
		return 0
	}
	return s.workerPool.AvailableSlots()
}

// GetQueueStats 获取队列统计信息
func (s *TemuTaskSubmitter) GetQueueStats() worker.QueueStats {
	if s.workerPool == nil {
		return worker.QueueStats{}
	}
	return s.workerPool.GetQueueStats()
}

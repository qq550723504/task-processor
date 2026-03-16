package temu

import (
	"context"
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/domain/model"
	"task-processor/internal/infra/worker"
	"task-processor/internal/pkg/jsonutil"
	"time"

	"github.com/sirupsen/logrus"
)

// TemuTaskSubmitter TEMU任务提交器（适配器）
type TemuTaskSubmitter struct {
	workerPool worker.WorkerPool
	logger     *logrus.Entry
}

// NewTemuTaskSubmitter 创建TEMU任务提交器
func NewTemuTaskSubmitter(workerPool worker.WorkerPool) *TemuTaskSubmitter {
	return &TemuTaskSubmitter{
		workerPool: workerPool,
		logger:     logger.GetGlobalLogger("temu_task_submitter"),
	}
}

// SubmitTask 提交任务
func (s *TemuTaskSubmitter) SubmitTask(ctx context.Context, taskData string) error {
	var task model.Task
	if err := jsonutil.UnmarshalString(taskData, &task, "解析任务数据失败"); err != nil {
		return err
	}

	fields := logrus.Fields{
		logger.FieldTaskID:    task.ID,
		logger.FieldProductID: task.ProductID,
		logger.FieldPlatform:  "temu",
		"priority":            task.Priority,
	}

	// 记录等待时间
	if task.CreateTime > 0 {
		waitTime := time.Since(time.Unix(task.CreateTime/1000, 0))
		fields["wait_time_ms"] = waitTime.Milliseconds()
		s.logger.WithFields(fields).Info("任务状态变更: Pending -> Processing")
	}

	// 提交到工作池
	if err := s.workerPool.Submit(worker.WorkerJob{
		TenantID: fmt.Sprintf("%d", task.TenantID),
		ShopID:   fmt.Sprintf("%d", task.StoreID),
		TaskData: taskData,
	}); err != nil {
		s.logger.WithError(err).WithFields(fields).Error("提交任务到工作池失败")
		return fmt.Errorf("提交任务到工作池失败: %w", err)
	}

	s.logger.WithFields(fields).Info("任务已提交到工作池")
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

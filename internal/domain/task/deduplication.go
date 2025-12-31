// Package task 提供任务去重功能
package task

import (
	"context"
	"sync"
	"time"

	"task-processor/internal/core/errors"
	"task-processor/internal/worker"

	"github.com/sirupsen/logrus"
)

// TaskState 任务状态
type TaskState string

const (
	TaskStatePending    TaskState = "pending"    // 待处理
	TaskStateProcessing TaskState = "processing" // 处理中
	TaskStateCompleted  TaskState = "completed"  // 已完成
	TaskStateFailed     TaskState = "failed"     // 失败
	TaskStateTimeout    TaskState = "timeout"    // 超时
)

// TaskRecord 任务记录
type TaskRecord struct {
	TaskID     string    `json:"task_id"`
	State      TaskState `json:"state"`
	SubmitTime time.Time `json:"submit_time"`
	UpdateTime time.Time `json:"update_time"`
	RetryCount int       `json:"retry_count"`
	MaxRetries int       `json:"max_retries"`
	Platform   string    `json:"platform"`
	Error      string    `json:"error,omitempty"`
}

// DeduplicationManager 去重管理器
type DeduplicationManager struct {
	tasks           map[string]*TaskRecord
	mu              sync.RWMutex
	logger          *logrus.Logger
	maxAge          time.Duration // 任务记录最大保存时间
	cleanupInterval time.Duration // 清理间隔
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewDeduplicationManager 创建去重管理器
func NewDeduplicationManager(logger *logrus.Logger, maxAge time.Duration) *DeduplicationManager {
	if maxAge <= 0 {
		maxAge = 24 * time.Hour // 默认24小时
	}

	return &DeduplicationManager{
		tasks:           make(map[string]*TaskRecord),
		logger:          logger,
		maxAge:          maxAge,
		cleanupInterval: time.Hour, // 每小时清理一次
	}
}

// Start 启动去重管理器
func (d *DeduplicationManager) Start(ctx context.Context) {
	d.ctx, d.cancel = context.WithCancel(ctx)

	// 启动清理goroutine
	go d.cleanupLoop()

	d.logger.Info("任务去重管理器启动完成")
}

// Stop 停止去重管理器
func (d *DeduplicationManager) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
	d.logger.Info("任务去重管理器停止完成")
}

// CanSubmitTask 检查任务是否可以提交
func (d *DeduplicationManager) CanSubmitTask(taskID string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	record, exists := d.tasks[taskID]
	if !exists {
		return true, nil // 新任务，可以提交
	}

	switch record.State {
	case TaskStateProcessing:
		// 检查是否超时
		if time.Since(record.UpdateTime) > 30*time.Minute {
			d.logger.Warnf("任务 %s 处理超时，允许重新提交", taskID)
			return true, nil
		}
		return false, errors.Newf(errors.ErrCodeTaskProcessing, "任务 %s 正在处理中", taskID)

	case TaskStateCompleted:
		return false, errors.Newf(errors.ErrCodeTaskDuplicate, "任务 %s 已完成", taskID)

	case TaskStateFailed:
		// 检查重试次数
		if record.RetryCount >= record.MaxRetries {
			return false, errors.Newf(errors.ErrCodeTaskDuplicate, "任务 %s 已达到最大重试次数", taskID)
		}
		return true, nil // 可以重试

	case TaskStateTimeout:
		return true, nil // 超时任务可以重新提交

	default:
		return true, nil
	}
}

// MarkTaskAsProcessing 标记任务为处理中
func (d *DeduplicationManager) MarkTaskAsProcessing(taskID, platform string, maxRetries int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()

	if record, exists := d.tasks[taskID]; exists {
		// 更新现有记录
		record.State = TaskStateProcessing
		record.UpdateTime = now
		record.Platform = platform
		if record.State == TaskStateFailed {
			record.RetryCount++
		}
		d.logger.Infof("更新任务状态为处理中: TaskID=%s, Platform=%s, RetryCount=%d",
			taskID, platform, record.RetryCount)
	} else {
		// 创建新记录
		d.tasks[taskID] = &TaskRecord{
			TaskID:     taskID,
			State:      TaskStateProcessing,
			SubmitTime: now,
			UpdateTime: now,
			RetryCount: 0,
			MaxRetries: maxRetries,
			Platform:   platform,
		}
		d.logger.Infof("标记新任务为处理中: TaskID=%s, Platform=%s", taskID, platform)
	}

	return nil
}

// MarkTaskAsCompleted 标记任务为已完成
func (d *DeduplicationManager) MarkTaskAsCompleted(taskID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if record, exists := d.tasks[taskID]; exists {
		record.State = TaskStateCompleted
		record.UpdateTime = time.Now()
		record.Error = ""
		d.logger.Infof("标记任务为已完成: TaskID=%s", taskID)
	}
}

// MarkTaskAsFailed 标记任务为失败
func (d *DeduplicationManager) MarkTaskAsFailed(taskID string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if record, exists := d.tasks[taskID]; exists {
		record.State = TaskStateFailed
		record.UpdateTime = time.Now()
		if err != nil {
			record.Error = err.Error()
		}
		d.logger.Infof("标记任务为失败: TaskID=%s, Error=%s", taskID, record.Error)
	}
}

// MarkTaskAsTimeout 标记任务为超时
func (d *DeduplicationManager) MarkTaskAsTimeout(taskID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if record, exists := d.tasks[taskID]; exists {
		record.State = TaskStateTimeout
		record.UpdateTime = time.Now()
		d.logger.Infof("标记任务为超时: TaskID=%s", taskID)
	}
}

// GetTaskRecord 获取任务记录
func (d *DeduplicationManager) GetTaskRecord(taskID string) (*TaskRecord, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	record, exists := d.tasks[taskID]
	if !exists {
		return nil, false
	}

	// 返回副本，避免外部修改
	return &TaskRecord{
		TaskID:     record.TaskID,
		State:      record.State,
		SubmitTime: record.SubmitTime,
		UpdateTime: record.UpdateTime,
		RetryCount: record.RetryCount,
		MaxRetries: record.MaxRetries,
		Platform:   record.Platform,
		Error:      record.Error,
	}, true
}

// GetTaskStats 获取任务统计
func (d *DeduplicationManager) GetTaskStats() map[string]int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := map[string]int{
		"total":      0,
		"pending":    0,
		"processing": 0,
		"completed":  0,
		"failed":     0,
		"timeout":    0,
	}

	for _, record := range d.tasks {
		stats["total"]++
		stats[string(record.State)]++
	}

	return stats
}

// cleanupLoop 清理循环
func (d *DeduplicationManager) cleanupLoop() {
	ticker := time.NewTicker(d.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			d.logger.Info("任务去重清理循环停止")
			return
		case <-ticker.C:
			d.cleanup()
		}
	}
}

// cleanup 清理过期任务记录
func (d *DeduplicationManager) cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	var toDelete []string

	for taskID, record := range d.tasks {
		// 清理条件：
		// 1. 已完成的任务超过最大保存时间
		// 2. 失败的任务超过最大保存时间且已达到最大重试次数
		// 3. 超时的任务超过最大保存时间
		shouldDelete := false

		switch record.State {
		case TaskStateCompleted:
			if now.Sub(record.UpdateTime) > d.maxAge {
				shouldDelete = true
			}
		case TaskStateFailed:
			if record.RetryCount >= record.MaxRetries && now.Sub(record.UpdateTime) > d.maxAge {
				shouldDelete = true
			}
		case TaskStateTimeout:
			if now.Sub(record.UpdateTime) > d.maxAge {
				shouldDelete = true
			}
		case TaskStateProcessing:
			// 处理中的任务如果超过2小时没有更新，标记为超时
			if now.Sub(record.UpdateTime) > 2*time.Hour {
				record.State = TaskStateTimeout
				record.UpdateTime = now
				d.logger.Warnf("任务处理超时，标记为超时: TaskID=%s", taskID)
			}
		}

		if shouldDelete {
			toDelete = append(toDelete, taskID)
		}
	}

	// 删除过期记录
	for _, taskID := range toDelete {
		delete(d.tasks, taskID)
	}

	if len(toDelete) > 0 {
		d.logger.Infof("清理了 %d 个过期任务记录", len(toDelete))
	}
}

// TransactionalTaskSubmitter 事务性任务提交器
type TransactionalTaskSubmitter struct {
	deduplicationManager *DeduplicationManager
	actualSubmitter      worker.TaskSubmitter
	logger               *logrus.Logger
}

// NewTransactionalTaskSubmitter 创建事务性任务提交器
func NewTransactionalTaskSubmitter(
	deduplicationManager *DeduplicationManager,
	actualSubmitter worker.TaskSubmitter,
	logger *logrus.Logger,
) *TransactionalTaskSubmitter {
	return &TransactionalTaskSubmitter{
		deduplicationManager: deduplicationManager,
		actualSubmitter:      actualSubmitter,
		logger:               logger,
	}
}

// SubmitTask 事务性提交任务
func (t *TransactionalTaskSubmitter) SubmitTask(ctx context.Context, taskData string) error {
	// 从taskData中提取taskID（这里假设taskData包含taskID信息）
	taskID := taskData // 简化处理，实际应该解析taskData获取taskID

	// 第一步：检查是否可以提交
	canSubmit, err := t.deduplicationManager.CanSubmitTask(taskID)
	if err != nil {
		return err
	}

	if !canSubmit {
		return errors.Newf(errors.ErrCodeTaskDuplicate, "任务 %s 不能提交", taskID)
	}

	// 第二步：标记为处理中
	if err := t.deduplicationManager.MarkTaskAsProcessing(taskID, "unknown", 3); err != nil {
		return errors.Wrap(err, errors.ErrCodeSystem, "标记任务为处理中失败")
	}

	// 第三步：实际提交任务
	if err := t.actualSubmitter.SubmitTask(ctx, taskData); err != nil {
		// 提交失败，回滚状态
		t.deduplicationManager.MarkTaskAsFailed(taskID, err)
		return errors.Wrap(err, errors.ErrCodeSystem, "提交任务失败")
	}

	t.logger.Infof("事务性任务提交成功: TaskID=%s", taskID)
	return nil
}

// GetQueueStats 获取队列统计（委托给实际提交器）
func (t *TransactionalTaskSubmitter) GetQueueStats() worker.QueueStats {
	return t.actualSubmitter.GetQueueStats()
}

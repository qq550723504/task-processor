package amazon

import (
	"context"
	"fmt"
	"task-processor/common/management/api"
	"task-processor/common/types"
	"time"

	"github.com/sirupsen/logrus"
)

// TaskHandler Amazon任务处理器
type TaskHandler struct {
	processor *AmazonProcessor
	logger    *logrus.Entry
}

// NewTaskHandler 创建Amazon任务处理器
func NewTaskHandler(processor *AmazonProcessor) *TaskHandler {
	return &TaskHandler{
		processor: processor,
		logger: logrus.WithFields(logrus.Fields{
			"component": "TaskHandler",
			"platform":  "amazon",
		}),
	}
}

// ProcessTask 处理任务
func (h *TaskHandler) ProcessTask(ctx context.Context, task types.Task, pipeline *Pipeline) error {
	h.logger.Infof("开始处理任务: ID=%s, ProductID=%s", task.ID, task.ProductID)

	// 创建任务上下文
	taskCtx := h.createTaskContext(ctx, &task)

	// 记录开始时间
	startTime := time.Now()

	// 执行管道处理
	if err := pipeline.Process(taskCtx); err != nil {
		h.logger.Errorf("任务处理失败: %v", err)
		h.handleTaskFailure(task, err)
		return err
	}

	// 记录处理时间
	processTime := time.Since(startTime)

	h.logger.Infof("任务处理成功: ID=%s, 耗时=%v", task.ID, processTime)
	h.updateTaskStatusSync(task.ID, types.TaskStatusPublished, "")

	return nil
}

// createTaskContext 创建任务上下文
func (h *TaskHandler) createTaskContext(ctx context.Context, task *types.Task) *TaskContext {
	return &TaskContext{
		Context: ctx,
		Task:    task,
		Data:    make(map[string]interface{}),
	}
}

// handleTaskFailure 处理任务失败
func (h *TaskHandler) handleTaskFailure(task types.Task, err error) {
	isRetryable := h.isRetryableError(err)
	h.logger.Debugf("错误分析: 类型=%T, 可重试=%t", err, isRetryable)

	if !isRetryable {
		h.updateTaskStatusSync(task.ID, types.TaskStatusTerminated, err.Error())
		h.logger.Errorf("❌ 任务处理失败且不可重试: ID=%s, 错误=%v", task.ID, err)
		return
	}

	task.RetryCount++
	maxRetries := h.processor.config.Processor.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	if task.RetryCount >= maxRetries {
		h.updateTaskStatusSync(task.ID, types.TaskStatusTerminated, err.Error())
		h.logger.Errorf("❌ 任务达到最大重试次数: ID=%s, 重试=%d", task.ID, task.RetryCount)
	} else {
		h.updateTaskStatusSync(task.ID, types.TaskStatusPendingRetry, err.Error())
		h.logger.Warnf("⚠️ 任务等待重试: ID=%s, 重试=%d", task.ID, task.RetryCount)
	}
}

// isRetryableError 判断错误是否可重试
func (h *TaskHandler) isRetryableError(err error) bool {
	// TODO: 实现Amazon特定的错误判断逻辑
	return true
}

// updateTaskStatusSync 同步更新任务状态
func (h *TaskHandler) updateTaskStatusSync(taskID string, status types.TaskStatus, errorMsg string) {
	var id int64
	if _, err := fmt.Sscanf(taskID, "%d", &id); err != nil {
		h.logger.Errorf("解析任务ID失败: %v", err)
		return
	}

	managementClient := h.processor.GetManagementClient()
	if managementClient == nil {
		h.logger.Error("管理系统客户端未初始化")
		return
	}

	importTaskClient := managementClient.GetImportTaskClient()
	if importTaskClient == nil {
		h.logger.Error("导入任务客户端未初始化")
		return
	}

	// 构建更新请求
	req := &api.ProductImportTaskUpdateReqDTO{
		ID:           id,
		Status:       status.Int16(),
		ErrorMessage: errorMsg,
	}

	// 重试机制
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		if err := importTaskClient.UpdateTaskStatus(req); err != nil {
			if i < maxRetries-1 {
				h.logger.Warnf("更新任务状态失败，重试 (%d/%d): %v", i+1, maxRetries, err)
				time.Sleep(time.Second * time.Duration(i+1))
				continue
			}
			h.logger.Errorf("❌ 更新任务状态失败: TaskID=%s, Error=%v", taskID, err)
			return
		}
		h.logger.Infof("✅ 任务状态更新成功: TaskID=%s, Status=%s", taskID, status.String())
		return
	}
}

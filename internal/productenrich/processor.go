// package productenrich 提供产品JSON生成的应用层实现
package productenrich

import (
	"context"
	"errors"
	"fmt"

	"task-processor/internal/infra/worker"

	"github.com/sirupsen/logrus"
)

// Processor 实现 worker.Processor 接口，将 ProductService 接入 infra/worker.Pool。
type Processor struct {
	service    ProductService
	taskRepo   TaskRepository
	logger     *logrus.Logger
	maxRetries int
}

// NewProcessor 创建 Processor 实例。
func NewProcessor(service ProductService, taskRepo TaskRepository, logger *logrus.Logger, maxRetries int) (*Processor, error) {
	if service == nil {
		return nil, fmt.Errorf("service cannot be nil")
	}
	if taskRepo == nil {
		return nil, fmt.Errorf("task repo cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &Processor{
		service:    service,
		taskRepo:   taskRepo,
		logger:     logger,
		maxRetries: maxRetries,
	}, nil
}

// errNoRetry 标记不应重试的错误（如数据质量拒绝）
type errNoRetry struct {
	cause error
}

func (e *errNoRetry) Error() string { return e.cause.Error() }
func (e *errNoRetry) Unwrap() error { return e.cause }

// isNoRetryError 判断错误是否为不可重试类型
func isNoRetryError(err error) bool {
	var e *errNoRetry
	return errors.As(err, &e)
}

// Start 实现 worker.Processor 接口，无需额外初始化。
func (p *Processor) Start(_ context.Context) error {
	p.logger.Info("productenrich processor started")
	return nil
}

// Close 实现 worker.Processor 接口，无需额外清理。
func (p *Processor) Close(_ context.Context) {}

// ProcessTask 实现 worker.Processor 接口。
// job.TaskData 存放 taskID（string），由 CreateGenerateTask 写入队列时设置。
func (p *Processor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	taskID := job.TaskData
	if taskID == "" {
		return fmt.Errorf("empty task ID in job")
	}

	log := p.logger.WithField("task_id", taskID)

	task, err := p.taskRepo.GetTask(ctx, taskID)
	if err != nil {
		log.WithError(err).Error("failed to get task")
		return fmt.Errorf("get task %s: %w", taskID, err)
	}

	if _, err := p.service.ProcessProduct(ctx, task); err != nil {
		log.WithError(err).Error("task processing failed")

		// 拒绝类错误（数据质量不足）不重试，直接保持 failed 状态
		if isNoRetryError(err) {
			log.Info("task rejected due to data quality, no retry")
			return err
		}

		// 递增重试次数
		if retryErr := p.taskRepo.IncrementRetryCount(ctx, taskID); retryErr != nil {
			log.WithError(retryErr).Warn("failed to increment retry count")
			return err
		}

		// 重新读取最新 retry_count
		updated, getErr := p.taskRepo.GetTask(ctx, taskID)
		if getErr != nil {
			log.WithError(getErr).Warn("failed to get updated task for retry check")
			return err
		}

		if updated.RetryCount < p.maxRetries {
			// 只重置 status 为 pending，保留 error 字段供查询
			if resetErr := p.taskRepo.ResetForRetry(ctx, taskID); resetErr != nil {
				log.WithError(resetErr).Warn("failed to reset task for retry")
			} else {
				log.WithField("retry_count", updated.RetryCount).Info("task queued for retry")
			}
		} else {
			log.WithField("retry_count", updated.RetryCount).Warn("task exceeded max retries, keeping failed")
		}

		return err
	}

	log.Info("task processed successfully")
	return nil
}

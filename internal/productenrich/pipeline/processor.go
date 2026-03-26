package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"
)

type Processor struct {
	service       productenrich.ProductService
	taskRepo      productenrich.TaskRepository
	taskSubmitter productenrich.TaskSubmitter
	stateMachine  *TaskStateMachine
	logger        *logrus.Logger
	maxRetries    int
}

func NewProcessor(service productenrich.ProductService, taskRepo productenrich.TaskRepository, logger *logrus.Logger, maxRetries int) (*Processor, error) {
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
		service:      service,
		taskRepo:     taskRepo,
		stateMachine: NewTaskStateMachine(maxRetries),
		logger:       logger,
		maxRetries:   maxRetries,
	}, nil
}

func (p *Processor) SetTaskSubmitter(submitter productenrich.TaskSubmitter) {
	p.taskSubmitter = submitter
}

func (p *Processor) Start(_ context.Context) error {
	p.logger.Info("productenrich processor started")
	return nil
}

func (p *Processor) Close(_ context.Context) {}

func (p *Processor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	taskID := job.TaskData
	if taskID == "" {
		return fmt.Errorf("empty task ID in job")
	}

	startedAt := time.Now()
	log := p.logger.WithField("task_id", taskID)

	task, err := p.taskRepo.GetTask(ctx, taskID)
	if err != nil {
		log.WithError(err).Error("failed to get task")
		return fmt.Errorf("get task %s: %w", taskID, err)
	}
	if err := p.stateMachine.CanProcess(task); err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"duration_ms": time.Since(startedAt).Milliseconds(),
			"outcome":     "skipped",
			"status":      task.Status,
		}).Info("skipping task due to non-processable status")
		return nil
	}

	if _, err := p.service.ProcessProduct(ctx, task); err != nil {
		disposition := p.stateMachine.ClassifyFailure(err)
		log.WithError(err).WithFields(logrus.Fields{
			"duration_ms":         time.Since(startedAt).Milliseconds(),
			"failure_disposition": disposition,
			"retry_count":         task.RetryCount,
			"outcome":             "failed",
		}).Error("task processing failed")

		if disposition == productenrich.FailureDispositionNoRetry {
			log.Info("task rejected due to data quality, no retry")
			return err
		}

		if retryErr := p.taskRepo.IncrementRetryCount(ctx, taskID); retryErr != nil {
			log.WithError(retryErr).Warn("failed to increment retry count")
			return err
		}

		updated, getErr := p.taskRepo.GetTask(ctx, taskID)
		if getErr != nil {
			log.WithError(getErr).Warn("failed to get updated task for retry check")
			return err
		}

		if p.stateMachine.ShouldRetry(updated) {
			if resetErr := p.taskRepo.PrepareRetry(ctx, taskID); resetErr != nil {
				log.WithError(resetErr).Warn("failed to reset task for retry")
			} else if p.taskSubmitter != nil {
				if submitErr := p.taskSubmitter.Submit(taskID); submitErr != nil {
					log.WithError(submitErr).Warn("failed to resubmit task to worker pool")
					if markErr := p.taskRepo.MarkFailed(ctx, taskID, fmt.Sprintf("failed to resubmit task for retry: %v", submitErr)); markErr != nil {
						log.WithError(markErr).Warn("failed to mark task as failed after resubmit error")
					}
				} else {
					log.WithField("retry_count", updated.RetryCount).Info("task resubmitted for retry")
				}
			} else {
				log.Warn("task submitter not set, task reset to pending but not resubmitted")
				if markErr := p.taskRepo.MarkFailed(ctx, taskID, "task retry submitter is not configured"); markErr != nil {
					log.WithError(markErr).Warn("failed to mark task as failed when retry submitter is missing")
				}
			}
		} else {
			log.WithField("retry_count", updated.RetryCount).Warn("task exceeded max retries, keeping failed")
		}

		return err
	}

	log.WithFields(logrus.Fields{
		"duration_ms": time.Since(startedAt).Milliseconds(),
		"outcome":     "success",
		"retry_count": task.RetryCount,
	}).Info("task processed successfully")
	return nil
}

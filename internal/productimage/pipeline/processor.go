package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/internal/infra/worker"
	productimage "task-processor/internal/productimage"
)

type imageService interface {
	ProcessImages(ctx context.Context, task *productimage.Task) (*productimage.ImageProcessResult, error)
	SetTaskSubmitter(submitter productimage.TaskSubmitter)
}

type Processor struct {
	service       imageService
	taskRepo      productimage.TaskRepository
	logger        *logrus.Logger
	taskSubmitter productimage.TaskSubmitter
	stateMachine  *TaskStateMachine
}

func NewProcessor(service imageService, taskRepo productimage.TaskRepository, logger *logrus.Logger, maxRetries int) (*Processor, error) {
	if service == nil {
		return nil, fmt.Errorf("service cannot be nil")
	}
	if taskRepo == nil {
		return nil, fmt.Errorf("task repository cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	return &Processor{service: service, taskRepo: taskRepo, logger: logger, stateMachine: NewTaskStateMachine(maxRetries)}, nil
}

func (p *Processor) SetTaskSubmitter(submitter productimage.TaskSubmitter) {
	p.taskSubmitter = submitter
}

func (p *Processor) Start(_ context.Context) error {
	p.logger.Info("productimage processor started")
	return nil
}

func (p *Processor) Close(_ context.Context) {}

func (p *Processor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	taskID := job.TaskData
	if taskID == "" {
		return fmt.Errorf("task ID cannot be empty")
	}
	startedAt := time.Now()
	log := p.logger.WithField("task_id", taskID)

	task, err := p.taskRepo.GetTask(ctx, taskID)
	if err != nil {
		log.WithError(err).Error("failed to load productimage task")
		return err
	}
	if err := p.stateMachine.CanProcess(task); err != nil {
		log.WithError(err).WithField("status", task.Status).Warn("skipping productimage task")
		return err
	}

	if _, err := p.service.ProcessImages(ctx, task); err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"duration_ms": time.Since(startedAt).Milliseconds(),
			"outcome":     "failed",
			"retry_count": task.RetryCount,
		}).Error("productimage worker processing failed")
		if p.stateMachine.ClassifyFailure(err) == productimage.FailureDispositionNoRetry {
			return err
		}
		if !p.stateMachine.ShouldRetry(task) {
			return err
		}
		if incErr := p.taskRepo.IncrementRetryCount(ctx, taskID); incErr != nil {
			log.WithError(incErr).Error("failed to increment image task retry count")
			return incErr
		}
		if resetErr := p.taskRepo.PrepareRetry(ctx, taskID); resetErr != nil {
			log.WithError(resetErr).Error("failed to prepare image task retry")
			return resetErr
		}
		if p.taskSubmitter != nil {
			if submitErr := p.taskSubmitter.Submit(taskID); submitErr != nil {
				_ = p.taskRepo.MarkFailed(ctx, taskID, fmt.Sprintf("failed to resubmit task: %v", submitErr))
				log.WithError(submitErr).Error("failed to resubmit image task")
				return submitErr
			}
			log.WithFields(logrus.Fields{
				"duration_ms": time.Since(startedAt).Milliseconds(),
				"outcome":     "retry_scheduled",
			}).Warn("productimage task scheduled for retry")
		}
		return err
	}
	log.WithFields(logrus.Fields{
		"duration_ms": time.Since(startedAt).Milliseconds(),
		"outcome":     "success",
	}).Info("productimage worker processing succeeded")
	return nil
}

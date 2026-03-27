package amazonlisting

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"task-processor/internal/infra/worker"
)

type Processor struct {
	service       Service
	repo          Repository
	taskSubmitter TaskSubmitter
	logger        *logrus.Logger
	maxRetries    int
}

func NewProcessor(service Service, repo Repository, logger *logrus.Logger, maxRetries int) (*Processor, error) {
	if service == nil {
		return nil, fmt.Errorf("service cannot be nil")
	}
	if repo == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	if maxRetries <= 0 {
		maxRetries = 2
	}
	return &Processor{service: service, repo: repo, logger: logger, maxRetries: maxRetries}, nil
}

func (p *Processor) SetTaskSubmitter(submitter TaskSubmitter) {
	p.taskSubmitter = submitter
}

func (p *Processor) Start(_ context.Context) error { return nil }

func (p *Processor) Close(_ context.Context) {}

func (p *Processor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
	task, err := p.repo.GetTask(ctx, job.TaskData)
	if err != nil {
		return err
	}
	if task.Status != TaskStatusPending {
		return nil
	}
	if _, err := p.service.ProcessListing(ctx, task); err != nil {
		if task.RetryCount < p.maxRetries {
			_ = p.repo.IncrementRetryCount(ctx, task.ID)
			_ = p.repo.PrepareRetry(ctx, task.ID)
			if p.taskSubmitter != nil {
				_ = p.taskSubmitter.Submit(task.ID)
			}
		}
		return err
	}
	return nil
}

var _ worker.Processor = (*Processor)(nil)

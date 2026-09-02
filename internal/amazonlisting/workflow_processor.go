package amazonlisting

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	worker "task-processor/internal/platform/workerpool"
	"task-processor/internal/shared/aiidentity"
)

type Processor struct {
	service Service
	repo    Repository
	logger  *logrus.Logger
}

func NewProcessor(service Service, repo Repository, logger *logrus.Logger) (*Processor, error) {
	if service == nil {
		return nil, fmt.Errorf("service cannot be nil")
	}
	if repo == nil {
		return nil, fmt.Errorf("repository cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}
	return &Processor{service: service, repo: repo, logger: logger}, nil
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
	envelope, envelopeErr := task.ExecutionEnvelope()
	if envelopeErr != nil {
		return joinTaskFailurePersistenceError(envelopeErr, p.repo.MarkFailed(ctx, task.ID, "identity_integrity: "+envelopeErr.Error()))
	}
	ctx, envelopeErr = aiidentity.RestoreExecutionEnvelope(ctx, envelope, task.ID)
	if envelopeErr != nil {
		return joinTaskFailurePersistenceError(envelopeErr, p.repo.MarkFailed(ctx, task.ID, "identity_integrity: "+envelopeErr.Error()))
	}
	if _, err := p.service.ProcessListing(ctx, task); err != nil {
		if errors.Is(err, ErrTaskNotPending) {
			return nil
		}
		return err
	}
	return nil
}

var _ worker.Processor = (*Processor)(nil)

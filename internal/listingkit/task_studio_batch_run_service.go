package listingkit

import (
	"context"
	"fmt"

	studiodomain "task-processor/internal/listing/studio"
)

type taskStudioBatchRunServiceConfig struct {
	repo              StudioBatchRunRepository
	studioSessionRepo studioBatchSeedSessionRepository
	startRun          func(ctx context.Context, runID string) error
	runner            *studiodomain.BatchRunService
}

type taskStudioBatchRunService struct {
	repo              StudioBatchRunRepository
	studioSessionRepo studioBatchSeedSessionRepository
	startRun          func(ctx context.Context, runID string) error
	runner            *studiodomain.BatchRunService
}

func newTaskStudioBatchRunService(config taskStudioBatchRunServiceConfig) *taskStudioBatchRunService {
	service := &taskStudioBatchRunService{
		repo:              config.repo,
		studioSessionRepo: config.studioSessionRepo,
		startRun:          config.startRun,
		runner:            config.runner,
	}
	service.ensureRunner()
	return service
}

func (s *taskStudioBatchRunService) CreateStudioBatchRun(ctx context.Context, req *CreateStudioBatchRunRequest) (*StudioBatchRunRecord, []StudioBatchRunItemRecord, error) {
	s.ensureRunner()
	if s.runner == nil {
		return nil, nil, fmt.Errorf("studio batch run service is not configured")
	}
	run, items, err := s.runner.CreateBatchRun(ctx, adaptStudioBatchRunRequest(req))
	if err != nil {
		return nil, nil, adaptStudioBatchRunError(err)
	}
	return adaptStudioBatchRunRecord(run), adaptStudioBatchRunItems(items), nil
}

func (s *taskStudioBatchRunService) GetStudioBatchRun(ctx context.Context, runID string) (*StudioBatchRunRecord, error) {
	s.ensureRunner()
	if s.runner == nil {
		return nil, fmt.Errorf("studio batch run service is not configured")
	}
	run, err := s.runner.GetBatchRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	return adaptStudioBatchRunRecord(run), nil
}

func (s *taskStudioBatchRunService) ListStudioBatchRunItems(ctx context.Context, runID string) ([]StudioBatchRunItemRecord, error) {
	s.ensureRunner()
	if s.runner == nil {
		return nil, fmt.Errorf("studio batch run service is not configured")
	}
	items, err := s.runner.ListBatchRunItems(ctx, runID)
	if err != nil {
		return nil, err
	}
	return adaptStudioBatchRunItems(items), nil
}

func (s *taskStudioBatchRunService) CancelStudioBatchRun(ctx context.Context, runID string) error {
	s.ensureRunner()
	if s.runner == nil {
		return fmt.Errorf("studio batch run service is not configured")
	}
	return s.runner.CancelBatchRun(ctx, runID)
}

func (s *taskStudioBatchRunService) ensureRunner() {
	if s == nil || s.runner != nil {
		return
	}
	s.runner = newListingStudioBatchRunService(s.repo, s.studioSessionRepo, s.startRun)
}

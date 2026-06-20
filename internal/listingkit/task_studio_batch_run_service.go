package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	studiodomain "task-processor/internal/listing/studio"
)

type taskStudioBatchRunServiceConfig struct {
	repo              StudioBatchRunRepository
	batchRepo         StudioBatchRepository
	studioSessionRepo studioBatchSeedSessionRepository
	startRun          func(ctx context.Context, runID string) error
	runner            *studiodomain.BatchRunService
}

type taskStudioBatchRunService struct {
	repo              StudioBatchRunRepository
	batchRepo         StudioBatchRepository
	studioSessionRepo studioBatchSeedSessionRepository
	startRun          func(ctx context.Context, runID string) error
	runner            *studiodomain.BatchRunService
}

func newTaskStudioBatchRunService(config taskStudioBatchRunServiceConfig) *taskStudioBatchRunService {
	service := &taskStudioBatchRunService{
		repo:              config.repo,
		batchRepo:         config.batchRepo,
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
	return s.attachBatchDiagnostics(ctx, adaptStudioBatchRunItems(items)), nil
}

func (s *taskStudioBatchRunService) CancelStudioBatchRun(ctx context.Context, runID string) error {
	s.ensureRunner()
	if s.runner == nil {
		return fmt.Errorf("studio batch run service is not configured")
	}
	return s.runner.CancelBatchRun(ctx, runID)
}

func (s *taskStudioBatchRunService) RecoverStudioBatchRun(ctx context.Context, runID string) error {
	s.ensureRunner()
	if s == nil || s.repo == nil {
		return fmt.Errorf("studio batch run service is not configured")
	}
	if s.startRun == nil {
		return fmt.Errorf("studio batch run service is not configured")
	}
	run, err := s.repo.GetStudioBatchRun(ctx, strings.TrimSpace(runID))
	if err != nil {
		return err
	}
	switch run.Status {
	case StudioBatchRunStatusRunning, StudioBatchRunStatusSucceeded:
		return NewStudioBatchActionValidationError(fmt.Sprintf("run cannot be recovered from status %s", run.Status))
	}
	if run.CancelRequested {
		return NewStudioBatchActionValidationError("run cannot be recovered while cancellation is requested")
	}
	if err := resetStudioBatchRunForRecovery(ctx, s.repo, run.ID, time.Now().UTC()); err != nil {
		return err
	}
	return s.startRun(ctx, run.ID)
}

func (s *taskStudioBatchRunService) ensureRunner() {
	if s == nil || s.runner != nil {
		return
	}
	s.runner = newListingStudioBatchRunService(s.repo, s.studioSessionRepo, s.startRun)
}

func (s *taskStudioBatchRunService) attachBatchDiagnostics(ctx context.Context, items []StudioBatchRunItemRecord) []StudioBatchRunItemRecord {
	if s == nil || s.batchRepo == nil || len(items) == 0 {
		return items
	}
	for index := range items {
		detail, err := s.batchRepo.GetStudioBatchDetail(ctx, items[index].BatchID)
		if err != nil || detail == nil || detail.Batch == nil {
			continue
		}
		items[index].BatchStatus = resolveStudioBatchRunBatchStatus(detail)
		items[index].BatchLastError = resolveStudioBatchRunBatchLastError(detail)
	}
	return items
}

func resolveStudioBatchRunBatchStatus(detail *StudioBatchDetailGraph) StudioBatchStatus {
	if detail == nil || detail.Batch == nil {
		return ""
	}
	status := aggregateStudioBatchStatus(detail.Items)
	if status != "" && detail.Batch.Status != StudioBatchStatusTasksCreating && detail.Batch.Status != StudioBatchStatusTasksCreated {
		return status
	}
	return detail.Batch.Status
}

func resolveStudioBatchRunBatchLastError(detail *StudioBatchDetailGraph) string {
	if detail == nil {
		return ""
	}
	for _, item := range detail.Items {
		if trimmed := strings.TrimSpace(item.LastError); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

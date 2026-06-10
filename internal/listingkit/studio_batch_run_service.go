package listingkit

import (
	"context"
	"fmt"
)

type CreateStudioBatchRunRequest struct {
	BatchIDs []string `json:"batch_ids"`
}

type StudioBatchRunService interface {
	CreateStudioBatchRun(ctx context.Context, req *CreateStudioBatchRunRequest) (*StudioBatchRunRecord, []StudioBatchRunItemRecord, error)
	GetStudioBatchRun(ctx context.Context, runID string) (*StudioBatchRunRecord, error)
	ListStudioBatchRunItems(ctx context.Context, runID string) ([]StudioBatchRunItemRecord, error)
	CancelStudioBatchRun(ctx context.Context, runID string) error
}

func (s *service) CreateStudioBatchRun(ctx context.Context, req *CreateStudioBatchRunRequest) (*StudioBatchRunRecord, []StudioBatchRunItemRecord, error) {
	return s.taskStudioBatchRunOrDefault().CreateStudioBatchRun(ctx, req)
}

func (s *service) GetStudioBatchRun(ctx context.Context, runID string) (*StudioBatchRunRecord, error) {
	return s.taskStudioBatchRunOrDefault().GetStudioBatchRun(ctx, runID)
}

func (s *service) ListStudioBatchRunItems(ctx context.Context, runID string) ([]StudioBatchRunItemRecord, error) {
	return s.taskStudioBatchRunOrDefault().ListStudioBatchRunItems(ctx, runID)
}

func (s *service) CancelStudioBatchRun(ctx context.Context, runID string) error {
	return s.taskStudioBatchRunOrDefault().CancelStudioBatchRun(ctx, runID)
}

func (s *service) taskStudioBatchRunOrDefault() *taskStudioBatchRunService {
	if s.taskStudioBatchRun != nil {
		return s.taskStudioBatchRun
	}
	s.taskStudioBatchRun = newTaskStudioBatchRunService(buildTaskStudioBatchRunServiceConfig(s))
	return s.taskStudioBatchRun
}

func (s *service) startStudioBatchRun(ctx context.Context, runID string) error {
	coordinator := s.buildStudioBatchRunCoordinator()
	if coordinator == nil {
		return fmt.Errorf("studio batch run executor is not configured")
	}
	return coordinator.StartRun(ctx, runID)
}

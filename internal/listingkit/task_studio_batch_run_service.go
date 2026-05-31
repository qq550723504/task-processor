package listingkit

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type taskStudioBatchRunServiceConfig struct {
	repo              StudioBatchRunRepository
	studioSessionRepo StudioSessionRepository
	startRun          func(ctx context.Context, runID string) error
}

type taskStudioBatchRunService struct {
	repo              StudioBatchRunRepository
	studioSessionRepo StudioSessionRepository
	startRun          func(ctx context.Context, runID string) error
}

func newTaskStudioBatchRunService(config taskStudioBatchRunServiceConfig) *taskStudioBatchRunService {
	return &taskStudioBatchRunService{
		repo:              config.repo,
		studioSessionRepo: config.studioSessionRepo,
		startRun:          config.startRun,
	}
}

func (s *taskStudioBatchRunService) CreateStudioBatchRun(ctx context.Context, req *CreateStudioBatchRunRequest) (*StudioBatchRunRecord, []StudioBatchRunItemRecord, error) {
	if s.repo == nil {
		return nil, nil, fmt.Errorf("studio batch run repository is not configured")
	}
	if s.studioSessionRepo == nil {
		return nil, nil, fmt.Errorf("studio session repository is not configured")
	}
	if s.startRun == nil {
		return nil, nil, fmt.Errorf("studio batch run starter is not configured")
	}
	if req == nil || len(req.BatchIDs) == 0 {
		return nil, nil, fmt.Errorf("batch_ids is required")
	}

	batchIDs := make([]string, 0, len(req.BatchIDs))
	seenBatchIDs := make(map[string]struct{}, len(req.BatchIDs))
	for _, batchID := range req.BatchIDs {
		normalized := strings.TrimSpace(batchID)
		if normalized == "" {
			return nil, nil, fmt.Errorf("batch_ids is required")
		}
		if _, exists := seenBatchIDs[normalized]; exists {
			return nil, nil, fmt.Errorf("duplicate batch_id: %s", normalized)
		}
		seenBatchIDs[normalized] = struct{}{}
		session, err := s.studioSessionRepo.GetSession(ctx, normalized)
		if err != nil {
			return nil, nil, err
		}
		if session == nil || !session.SavedAsBatch {
			return nil, nil, ErrStudioSessionNotFound
		}
		batchIDs = append(batchIDs, normalized)
	}

	runID := uuid.NewString()
	run := &StudioBatchRunRecord{
		ID:            runID,
		UserID:        RequestUserIDFromContext(ctx),
		Mode:          StudioBatchRunModeGenerate,
		FailurePolicy: StudioBatchRunFailurePolicyContinueOnError,
		Status:        StudioBatchRunStatusPending,
		TotalBatches:  len(batchIDs),
	}
	items := make([]StudioBatchRunItemRecord, 0, len(batchIDs))
	for i, batchID := range batchIDs {
		items = append(items, StudioBatchRunItemRecord{
			ID:       fmt.Sprintf("%s:%d", runID, i+1),
			RunID:    runID,
			BatchID:  batchID,
			Position: i + 1,
			Status:   StudioBatchRunItemStatusPending,
		})
	}
	if err := s.repo.CreateStudioBatchRun(ctx, run, items); err != nil {
		return nil, nil, err
	}
	if err := s.startRun(ctx, run.ID); err != nil {
		return nil, nil, err
	}
	return run, items, nil
}

func (s *taskStudioBatchRunService) GetStudioBatchRun(ctx context.Context, runID string) (*StudioBatchRunRecord, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch run repository is not configured")
	}
	return s.repo.GetStudioBatchRun(ctx, strings.TrimSpace(runID))
}

func (s *taskStudioBatchRunService) ListStudioBatchRunItems(ctx context.Context, runID string) ([]StudioBatchRunItemRecord, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch run repository is not configured")
	}
	return s.repo.ListStudioBatchRunItems(ctx, strings.TrimSpace(runID))
}

func (s *taskStudioBatchRunService) CancelStudioBatchRun(ctx context.Context, runID string) error {
	if s.repo == nil {
		return fmt.Errorf("studio batch run repository is not configured")
	}
	run, err := s.repo.GetStudioBatchRun(ctx, strings.TrimSpace(runID))
	if err != nil {
		return err
	}
	run.CancelRequested = true
	return s.repo.UpdateStudioBatchRun(ctx, run)
}

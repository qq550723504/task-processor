package listingkit

import (
	"context"
	"errors"
	"strings"
	"time"

	studiodomain "task-processor/internal/listing/studio"

	"github.com/google/uuid"
)

func newListingStudioBatchRunService(
	repo StudioBatchRunRepository,
	studioSessionRepo studioBatchSeedSessionRepository,
	startRun func(ctx context.Context, runID string) error,
) *studiodomain.BatchRunService {
	return studiodomain.NewBatchRunService(studiodomain.BatchRunServiceConfig{
		Repo:          studioBatchRunRepositoryAdapter{repo: repo},
		SessionRepo:   studioBatchSeedSessionRepositoryAdapter{repo: studioSessionRepo},
		StartRun:      startRun,
		NewRunID:      uuid.NewString,
		RequestUserID: RequestUserIDFromContext,
	})
}

type listingStudioBatchRunCompletionRunner = studiodomain.BatchRunCompletionService[
	StudioBatchRunItemRecord,
	StudioBatchRunStatus,
]

func newListingStudioBatchRunCompletionService(repo StudioBatchRunRepository, now func() time.Time) *listingStudioBatchRunCompletionRunner {
	return studiodomain.NewBatchRunCompletionService(studiodomain.BatchRunCompletionServiceConfig[
		StudioBatchRunItemRecord,
		StudioBatchRunStatus,
	]{
		UpdateItem: func(ctx context.Context, item *StudioBatchRunItemRecord) error {
			if repo == nil {
				return errors.New("studio batch run repository is not configured")
			}
			return repo.UpdateStudioBatchRunItem(ctx, item)
		},
		ItemStatus: func(item *StudioBatchRunItemRecord) StudioBatchRunStatus {
			if item == nil {
				return ""
			}
			return StudioBatchRunStatus(item.Status)
		},
		MarkCancelled: func(item *StudioBatchRunItemRecord, at time.Time) {
			if item == nil {
				return
			}
			item.Status = StudioBatchRunItemStatusCancelled
			item.FinishedAt = &at
			item.UpdatedAt = at
		},
		Now:                      now,
		SucceededStatus:          StudioBatchRunStatusSucceeded,
		FailedStatus:             StudioBatchRunStatusFailed,
		CancelledStatus:          StudioBatchRunStatusCancelled,
		PartiallySucceededStatus: StudioBatchRunStatusPartiallySucceeded,
	})
}

type studioBatchRunRepositoryAdapter struct {
	repo StudioBatchRunRepository
}

func (a studioBatchRunRepositoryAdapter) CreateBatchRun(ctx context.Context, run *studiodomain.BatchRunRecord, items []studiodomain.BatchRunItemRecord) error {
	if a.repo == nil {
		return errors.New("studio batch run repository is not configured")
	}
	return a.repo.CreateStudioBatchRun(ctx, adaptDomainStudioBatchRunRecord(run), adaptDomainStudioBatchRunItems(items))
}

func (a studioBatchRunRepositoryAdapter) GetBatchRun(ctx context.Context, runID string) (*studiodomain.BatchRunRecord, error) {
	if a.repo == nil {
		return nil, errors.New("studio batch run repository is not configured")
	}
	run, err := a.repo.GetStudioBatchRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	return adaptStudioBatchRunRecordToDomain(run), nil
}

func (a studioBatchRunRepositoryAdapter) ListBatchRunItems(ctx context.Context, runID string) ([]studiodomain.BatchRunItemRecord, error) {
	if a.repo == nil {
		return nil, errors.New("studio batch run repository is not configured")
	}
	items, err := a.repo.ListStudioBatchRunItems(ctx, runID)
	if err != nil {
		return nil, err
	}
	return adaptStudioBatchRunItemsToDomain(items), nil
}

func (a studioBatchRunRepositoryAdapter) UpdateBatchRun(ctx context.Context, run *studiodomain.BatchRunRecord) error {
	if a.repo == nil {
		return errors.New("studio batch run repository is not configured")
	}
	return a.repo.UpdateStudioBatchRun(ctx, adaptDomainStudioBatchRunRecord(run))
}

type studioBatchSeedSessionRepositoryAdapter struct {
	repo studioBatchSeedSessionRepository
}

func (a studioBatchSeedSessionRepositoryAdapter) GetSession(ctx context.Context, batchID string) (*studiodomain.BatchSeedSession, error) {
	if a.repo == nil {
		return nil, errors.New("studio session repository is not configured")
	}
	session, err := a.repo.GetSession(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}
	return &studiodomain.BatchSeedSession{SavedAsBatch: session.SavedAsBatch}, nil
}

func adaptStudioBatchRunRequest(req *CreateStudioBatchRunRequest) *studiodomain.CreateBatchRunRequest {
	if req == nil {
		return nil
	}
	return &studiodomain.CreateBatchRunRequest{
		BatchIDs: append([]string(nil), req.BatchIDs...),
		Mode:     strings.TrimSpace(req.Mode),
	}
}

func adaptStudioBatchRunRecord(run *studiodomain.BatchRunRecord) *StudioBatchRunRecord {
	if run == nil {
		return nil
	}
	return &StudioBatchRunRecord{
		ID:              run.ID,
		UserID:          run.UserID,
		Mode:            StudioBatchRunMode(run.Mode),
		FailurePolicy:   StudioBatchRunFailurePolicy(run.FailurePolicy),
		Status:          StudioBatchRunStatus(run.Status),
		TotalBatches:    run.TotalBatches,
		CancelRequested: run.CancelRequested,
	}
}

func adaptStudioBatchRunItems(items []studiodomain.BatchRunItemRecord) []StudioBatchRunItemRecord {
	out := make([]StudioBatchRunItemRecord, 0, len(items))
	for _, item := range items {
		out = append(out, StudioBatchRunItemRecord{
			ID:       item.ID,
			RunID:    item.RunID,
			BatchID:  item.BatchID,
			Position: item.Position,
			Status:   StudioBatchRunItemStatus(item.Status),
		})
	}
	return out
}

func adaptDomainStudioBatchRunRecord(run *studiodomain.BatchRunRecord) *StudioBatchRunRecord {
	if run == nil {
		return nil
	}
	return &StudioBatchRunRecord{
		ID:              run.ID,
		UserID:          run.UserID,
		Mode:            StudioBatchRunMode(run.Mode),
		FailurePolicy:   StudioBatchRunFailurePolicy(run.FailurePolicy),
		Status:          StudioBatchRunStatus(run.Status),
		TotalBatches:    run.TotalBatches,
		CancelRequested: run.CancelRequested,
	}
}

func adaptDomainStudioBatchRunItems(items []studiodomain.BatchRunItemRecord) []StudioBatchRunItemRecord {
	out := make([]StudioBatchRunItemRecord, 0, len(items))
	for _, item := range items {
		out = append(out, StudioBatchRunItemRecord{
			ID:       item.ID,
			RunID:    item.RunID,
			BatchID:  item.BatchID,
			Position: item.Position,
			Status:   StudioBatchRunItemStatus(item.Status),
		})
	}
	return out
}

func adaptStudioBatchRunRecordToDomain(run *StudioBatchRunRecord) *studiodomain.BatchRunRecord {
	if run == nil {
		return nil
	}
	return &studiodomain.BatchRunRecord{
		ID:              run.ID,
		UserID:          run.UserID,
		Mode:            string(run.Mode),
		FailurePolicy:   string(run.FailurePolicy),
		Status:          string(run.Status),
		TotalBatches:    run.TotalBatches,
		CancelRequested: run.CancelRequested,
	}
}

func adaptStudioBatchRunItemsToDomain(items []StudioBatchRunItemRecord) []studiodomain.BatchRunItemRecord {
	out := make([]studiodomain.BatchRunItemRecord, 0, len(items))
	for _, item := range items {
		out = append(out, studiodomain.BatchRunItemRecord{
			ID:       item.ID,
			RunID:    item.RunID,
			BatchID:  item.BatchID,
			Position: item.Position,
			Status:   string(item.Status),
		})
	}
	return out
}

func adaptStudioBatchRunError(err error) error {
	if errors.Is(err, studiodomain.ErrBatchSessionNotFound) {
		return ErrStudioSessionNotFound
	}
	return err
}

package listingkit

import (
	"context"
	"fmt"

	studiodomain "task-processor/internal/listing/studio"
)

type listingStudioBatchGenerationRunner = studiodomain.BatchGenerationService[
	StudioBatchDetail,
	CreateStudioBatchTasksResult,
]

func newListingStudioBatchGenerationService(s *taskStudioBatchService) *listingStudioBatchGenerationRunner {
	return studiodomain.NewBatchGenerationService(studiodomain.BatchGenerationServiceConfig[
		StudioBatchDetail,
		CreateStudioBatchTasksResult,
	]{
		RefreshGraph: func(ctx context.Context, batchID string) error {
			if s == nil || s.repo == nil {
				return fmt.Errorf("studio batch repository is not configured")
			}
			return s.refreshStudioBatchGenerationGraph(ctx, batchID)
		},
		EnsureGraphForResume: func(ctx context.Context, batchID string) error {
			if s == nil || s.repo == nil {
				return fmt.Errorf("studio batch repository is not configured")
			}
			return s.ensureStudioBatchGenerationGraphForResume(ctx, batchID)
		},
		ContinueGeneration: func(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
			return s.continueStudioBatchGeneration(ctx, batchID)
		},
		LoadDetail: func(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
			if s == nil || s.repo == nil {
				return nil, fmt.Errorf("studio batch repository is not configured")
			}
			return s.GetStudioBatchDetail(ctx, batchID)
		},
		ShouldResumeTaskCreation: func(ctx context.Context, batchID string) bool {
			if s == nil {
				return false
			}
			return shouldResumeStudioBatchTaskCreation(ctx, s.repo, batchID)
		},
		ResumeTaskCreation: func(ctx context.Context, batchID string) (*CreateStudioBatchTasksResult, error) {
			if s == nil || s.repo == nil {
				return nil, fmt.Errorf("studio batch repository is not configured")
			}
			return s.resumeStudioBatchTaskCreation(ctx, batchID)
		},
		AdaptResumeResult: adaptCreateStudioBatchTasksResultToDetail,
		PrepareRetryItems: func(ctx context.Context, batchID string, itemIDs []string) (*StudioBatchDetail, error) {
			if s == nil || s.repo == nil {
				return nil, fmt.Errorf("studio batch repository is not configured")
			}
			if s.generator == nil {
				return nil, fmt.Errorf("studio batch generator is not configured")
			}
			if err := s.syncStudioBatchRetryExecutionConfigFromDraft(ctx, batchID); err != nil {
				return nil, err
			}
			s.ensureRetryRunner()
			if s.retryRunner == nil {
				return nil, fmt.Errorf("studio batch retry prepare service is not configured")
			}
			return s.retryRunner.PrepareRetryItems(ctx, batchID, itemIDs)
		},
	})
}

func adaptCreateStudioBatchTasksResultToDetail(result *CreateStudioBatchTasksResult) *StudioBatchDetail {
	if result == nil {
		return nil
	}
	return &StudioBatchDetail{
		Batch:        result.Batch,
		Items:        result.Items,
		CreatedTasks: result.CreatedTasks,
		FailedTasks:  result.FailedTasks,
	}
}

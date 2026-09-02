package listingkit

import (
	"context"
	"fmt"

	studiodomain "task-processor/internal/listing/studio"
)

type listingStudioBatchGenerationRunner = studiodomain.BatchGenerationService[StudioBatchDetail]

func newListingStudioBatchGenerationService(s *taskStudioBatchService) *listingStudioBatchGenerationRunner {
	return studiodomain.NewBatchGenerationService(studiodomain.BatchGenerationServiceConfig[StudioBatchDetail]{
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

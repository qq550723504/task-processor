package listingkit

import (
	"context"
	"fmt"

	studiodomain "task-processor/internal/listing/studio"
)

type listingStudioBatchServiceRunner = studiodomain.BatchService[
	StudioBatchDetail,
	ApproveStudioBatchDesignsRequest,
	RetryStudioBatchItemsRequest,
]

func newListingStudioBatchServiceRunner(s *taskStudioBatchService) *listingStudioBatchServiceRunner {
	return studiodomain.NewBatchService(studiodomain.BatchServiceConfig[
		StudioBatchDetail,
		ApproveStudioBatchDesignsRequest,
		RetryStudioBatchItemsRequest,
	]{
		GetDetail: func(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
			s.ensureDetailRunner()
			if s == nil || s.detailRunner == nil {
				return nil, fmt.Errorf("studio batch repository is not configured")
			}
			return s.detailRunner.GetDetail(ctx, batchID)
		},
		StartGeneration: func(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
			s.ensureBatchRunner()
			if s == nil || s.batchRunner == nil {
				return nil, fmt.Errorf("studio batch generation runner is not configured")
			}
			return s.batchRunner.StartGeneration(ctx, batchID)
		},
		PrepareGeneration: func(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
			s.ensureBatchRunner()
			if s == nil || s.batchRunner == nil {
				return nil, fmt.Errorf("studio batch generation runner is not configured")
			}
			return s.batchRunner.PrepareGeneration(ctx, batchID)
		},
		ResumeGeneration: func(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
			s.ensureBatchRunner()
			if s == nil || s.batchRunner == nil {
				return nil, fmt.Errorf("studio batch generation runner is not configured")
			}
			return s.batchRunner.ResumeGeneration(ctx, batchID)
		},
		ApproveDesigns: func(ctx context.Context, batchID string, designIDs []string) (*StudioBatchDetail, error) {
			s.ensureReviewRunner()
			if s == nil || s.reviewRunner == nil {
				return nil, fmt.Errorf("studio batch repository is not configured")
			}
			return s.reviewRunner.ApproveDesigns(ctx, batchID, designIDs)
		},
		ApprovedDesignIDs: func(req *ApproveStudioBatchDesignsRequest) []string {
			if req == nil {
				return normalizeStudioBatchDesignIDs(nil)
			}
			return normalizeStudioBatchDesignIDs(req.DesignIDs)
		},
		RetryItems: func(ctx context.Context, batchID string, itemIDs []string) (*StudioBatchDetail, error) {
			s.ensureBatchRunner()
			if s == nil || s.batchRunner == nil {
				return nil, fmt.Errorf("studio batch generation runner is not configured")
			}
			return s.batchRunner.RetryItems(ctx, batchID, itemIDs)
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
		RetryItemIDs: func(req *RetryStudioBatchItemsRequest) []string {
			if req == nil {
				return normalizeStudioBatchItemIDs(nil)
			}
			return normalizeStudioBatchItemIDs(req.ItemIDs)
		},
	})
}

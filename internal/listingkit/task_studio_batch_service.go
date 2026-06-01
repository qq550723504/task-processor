package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type taskStudioBatchService struct {
	repo        StudioBatchRepository
	currentTime func() time.Time
}

func newTaskStudioBatchService(config taskStudioBatchServiceConfig) *taskStudioBatchService {
	return &taskStudioBatchService{
		repo:        config.repo,
		currentTime: time.Now,
	}
}

func (s *taskStudioBatchService) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	detail, err := s.repo.GetStudioBatchDetail(ctx, strings.TrimSpace(batchID))
	if err != nil {
		return nil, err
	}
	return projectStudioBatchDetail(detail), nil
}

func (s *taskStudioBatchService) ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}

	normalizedBatchID := strings.TrimSpace(batchID)
	if _, err := s.repo.GetStudioBatchDetail(ctx, normalizedBatchID); err != nil {
		return nil, err
	}

	approvedIDs := normalizeStudioBatchDesignIDs(nil)
	if req != nil {
		approvedIDs = normalizeStudioBatchDesignIDs(req.DesignIDs)
	}
	if err := s.repo.ReplaceStudioMaterializedDesignReviews(ctx, normalizedBatchID, approvedIDs, s.currentTime().UTC()); err != nil {
		return nil, err
	}

	return s.GetStudioBatchDetail(ctx, normalizedBatchID)
}

func projectStudioBatchDetail(detail *StudioBatchDetailGraph) *StudioBatchDetail {
	if detail == nil {
		return &StudioBatchDetail{}
	}

	items := make([]StudioBatchItemDetail, 0, len(detail.Items))
	for _, item := range detail.Items {
		items = append(items, StudioBatchItemDetail{
			Item:     item,
			Attempts: append([]StudioGenerationAttemptRecord(nil), detail.AttemptsByItem[item.ID]...),
			Designs:  append([]StudioMaterializedDesignRecord(nil), detail.DesignsByItem[item.ID]...),
		})
	}

	return &StudioBatchDetail{
		Batch: detail.Batch,
		Items: items,
	}
}

func normalizeStudioBatchDesignIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	result := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		normalized := strings.TrimSpace(id)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

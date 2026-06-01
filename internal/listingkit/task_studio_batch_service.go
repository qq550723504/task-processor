package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type taskStudioBatchService struct {
	repo              StudioBatchRepository
	studioSessionRepo StudioSessionRepository
	generator         studioBatchGenerator
	currentTime       func() time.Time
}

func newTaskStudioBatchService(config taskStudioBatchServiceConfig) *taskStudioBatchService {
	return &taskStudioBatchService{
		repo:              config.repo,
		studioSessionRepo: config.studioSessionRepo,
		generator:         config.generator,
		currentTime:       time.Now,
	}
}

func (s *taskStudioBatchService) StartStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	if s.generator == nil {
		return nil, fmt.Errorf("studio batch generator is not configured")
	}

	normalizedBatchID := strings.TrimSpace(batchID)
	if err := s.ensureStudioBatchGenerationGraph(ctx, normalizedBatchID); err != nil {
		return nil, err
	}
	if err := s.generator.RunPendingStudioBatchItems(ctx, normalizedBatchID); err != nil {
		return nil, err
	}
	if err := s.generator.RecoverStudioBatchMaterialization(ctx, normalizedBatchID); err != nil {
		return nil, err
	}
	return s.GetStudioBatchDetail(ctx, normalizedBatchID)
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

func (s *taskStudioBatchService) ensureStudioBatchGenerationGraph(ctx context.Context, batchID string) error {
	if _, err := s.repo.GetStudioBatch(ctx, batchID); err == nil {
		return s.ensureExpandedStudioBatchItems(ctx, batchID)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if s.studioSessionRepo == nil {
		return fmt.Errorf("studio session repository is not configured")
	}
	session, err := s.studioSessionRepo.GetSession(ctx, batchID)
	if err != nil {
		return err
	}
	if session == nil || !session.SavedAsBatch {
		return ErrStudioSessionNotFound
	}

	now := s.currentTime().UTC()
	batch := &StudioBatchRecord{
		ID:                    session.ID,
		Status:                StudioBatchStatusGenerating,
		Prompt:                session.Prompt,
		GroupedImageMode:      strings.TrimSpace(session.GroupedImageMode),
		Selection:             session.Selection,
		GroupedSelections:     append(SheinStudioGroupedSelectionList(nil), session.GroupedSelections...),
		StyleCount:            session.StyleCount,
		VariationIntensity:    session.VariationIntensity,
		ArtworkModel:          session.ArtworkModel,
		SelectedSDSImages:     append(SheinStudioSelectedSDSImageList(nil), session.SelectedSDSImages...),
		TransparentBackground: session.TransparentBackground,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if storeID, convErr := strconv.ParseInt(strings.TrimSpace(session.SheinStoreID), 10, 64); convErr == nil {
		batch.SheinStoreID = storeID
	}
	if groupedMode := strings.TrimSpace(session.GroupedImageMode); groupedMode == "per_product" || groupedMode == "shared_by_size" {
		batch.GroupedImageMode = groupedMode
	}
	if batch.GroupedImageMode == "" {
		batch.GroupedImageMode = "shared_by_size"
	}

	items := expandStudioBatchItems(batch)
	for index := range items {
		items[index].CreatedAt = now.Add(time.Duration(index) * time.Second)
		items[index].UpdatedAt = items[index].CreatedAt
	}
	return s.repo.CreateStudioBatchGraph(ctx, batch, items, nil, nil)
}

func (s *taskStudioBatchService) ensureExpandedStudioBatchItems(ctx context.Context, batchID string) error {
	detail, err := s.repo.GetStudioBatchDetail(ctx, batchID)
	if err != nil {
		return err
	}
	if detail == nil || detail.Batch == nil {
		return nil
	}
	if len(detail.Items) > 0 {
		batch := *detail.Batch
		if batch.Status == StudioBatchStatusDraft {
			batch.Status = StudioBatchStatusGenerating
			batch.UpdatedAt = s.currentTime().UTC()
			return s.repo.UpdateStudioBatch(ctx, &batch)
		}
		return nil
	}

	items := expandStudioBatchItems(detail.Batch)
	now := s.currentTime().UTC()
	for index := range items {
		items[index].CreatedAt = now.Add(time.Duration(index) * time.Second)
		items[index].UpdatedAt = items[index].CreatedAt
	}
	if len(items) > 0 {
		if err := s.repo.CreateStudioBatchItems(ctx, batchID, items); err != nil {
			return err
		}
	}

	batch := *detail.Batch
	batch.Status = StudioBatchStatusGenerating
	batch.UpdatedAt = now
	return s.repo.UpdateStudioBatch(ctx, &batch)
}

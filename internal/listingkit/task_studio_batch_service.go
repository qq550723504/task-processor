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
	if err := s.refreshStudioBatchGenerationGraph(ctx, normalizedBatchID); err != nil {
		return nil, err
	}
	return s.continueStudioBatchGeneration(ctx, normalizedBatchID)
}

func (s *taskStudioBatchService) PrepareStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	if s.generator == nil {
		return nil, fmt.Errorf("studio batch generator is not configured")
	}

	normalizedBatchID := strings.TrimSpace(batchID)
	if err := s.refreshStudioBatchGenerationGraph(ctx, normalizedBatchID); err != nil {
		return nil, err
	}
	return s.GetStudioBatchDetail(ctx, normalizedBatchID)
}

func (s *taskStudioBatchService) ResumeStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	if s.generator == nil {
		return nil, fmt.Errorf("studio batch generator is not configured")
	}

	normalizedBatchID := strings.TrimSpace(batchID)
	if err := s.ensureStudioBatchGenerationGraphForResume(ctx, normalizedBatchID); err != nil {
		return nil, err
	}
	return s.continueStudioBatchGeneration(ctx, normalizedBatchID)
}

func (s *taskStudioBatchService) continueStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	if err := s.generator.RecoverStudioBatchMaterialization(ctx, batchID); err != nil {
		return nil, err
	}
	if err := s.generator.RunPendingStudioBatchItems(ctx, batchID); err != nil {
		return nil, err
	}
	if err := s.generator.RecoverStudioBatchMaterialization(ctx, batchID); err != nil {
		return nil, err
	}
	return s.GetStudioBatchDetail(ctx, batchID)
}

func (s *taskStudioBatchService) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	detail, err := s.repo.GetStudioBatchDetail(ctx, normalizedBatchID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if syncErr := s.ensureStudioBatchGenerationGraphForResume(ctx, normalizedBatchID); syncErr != nil {
			return nil, syncErr
		}
		detail, err = s.repo.GetStudioBatchDetail(ctx, normalizedBatchID)
	}
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

func (s *taskStudioBatchService) RetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {
	detail, err := s.PrepareRetryStudioBatchItems(ctx, batchID, req)
	if err != nil {
		return nil, err
	}
	return s.continueStudioBatchGeneration(ctx, detail.Batch.ID)
}

func (s *taskStudioBatchService) PrepareRetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	if s.generator == nil {
		return nil, fmt.Errorf("studio batch generator is not configured")
	}

	normalizedBatchID := strings.TrimSpace(batchID)
	detail, err := s.repo.GetStudioBatchDetail(ctx, normalizedBatchID)
	if err != nil {
		return nil, err
	}

	itemIDs := normalizeStudioBatchItemIDs(nil)
	if req != nil {
		itemIDs = normalizeStudioBatchItemIDs(req.ItemIDs)
	}
	if len(itemIDs) == 0 {
		return nil, NewStudioBatchActionValidationError("item_ids is required")
	}

	itemsByID := make(map[string]StudioBatchItemRecord, len(detail.Items))
	for _, item := range detail.Items {
		itemsByID[item.ID] = item
	}
	itemsToRetry := make([]StudioBatchItemRecord, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		item, ok := itemsByID[itemID]
		if !ok {
			return nil, NewStudioBatchActionValidationError(fmt.Sprintf("unknown item_id: %s", itemID))
		}
		if !isStudioBatchItemRetryable(item.Status) {
			return nil, NewStudioBatchActionValidationError(fmt.Sprintf("item %s is not retryable from status %s", itemID, item.Status))
		}
		itemsToRetry = append(itemsToRetry, item)
	}

	now := s.currentTime().UTC()
	for _, item := range itemsToRetry {
		item.Status = StudioBatchItemStatusPending
		item.LastError = ""
		item.UpdatedAt = now
		if err := s.repo.UpdateStudioBatchItem(ctx, &item); err != nil {
			return nil, err
		}
	}

	return s.GetStudioBatchDetail(ctx, normalizedBatchID)
}

func (s *taskStudioBatchService) CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}

	normalizedBatchID := strings.TrimSpace(batchID)
	designIDs := normalizeStudioBatchDesignIDs(nil)
	if req != nil {
		designIDs = normalizeStudioBatchDesignIDs(req.DesignIDs)
	}
	if len(designIDs) == 0 {
		return nil, NewStudioBatchActionValidationError("design_ids is required")
	}

	designs, err := s.repo.ListStudioMaterializedDesignsByIDs(ctx, normalizedBatchID, designIDs)
	if err != nil {
		return nil, err
	}
	if len(designs) != len(designIDs) {
		return nil, gorm.ErrRecordNotFound
	}

	createdTasks := make([]SheinStudioCreatedTask, 0, len(designs))
	for _, design := range designs {
		if design.ReviewStatus != StudioMaterializedDesignReviewStatusApproved {
			return nil, NewStudioBatchActionValidationError(fmt.Sprintf("design %s is not approved", design.ID))
		}
		createdTasks = append(createdTasks, SheinStudioCreatedTask{
			ID:       fmt.Sprintf("%s:task:%s", normalizedBatchID, design.ID),
			Title:    firstNonEmpty(strings.TrimSpace(design.TargetGroupLabel), strings.TrimSpace(design.ID)),
			DesignID: design.ID,
		})
	}

	batch, err := s.repo.GetStudioBatch(ctx, normalizedBatchID)
	if err != nil {
		return nil, err
	}
	batch.Status = StudioBatchStatusTasksCreated
	batch.UpdatedAt = s.currentTime().UTC()
	if err := s.repo.UpdateStudioBatch(ctx, batch); err != nil {
		return nil, err
	}

	detail, err := s.GetStudioBatchDetail(ctx, normalizedBatchID)
	if err != nil {
		return nil, err
	}
	return &CreateStudioBatchTasksResult{
		Batch:        detail.Batch,
		Items:        detail.Items,
		CreatedTasks: createdTasks,
	}, nil
}

func projectStudioBatchDetail(detail *StudioBatchDetailGraph) *StudioBatchDetail {
	if detail == nil {
		return &StudioBatchDetail{}
	}

	batch := projectStudioBatchRecord(detail.Batch, detail.Items)
	items := make([]StudioBatchItemDetail, 0, len(detail.Items))
	for _, item := range detail.Items {
		items = append(items, StudioBatchItemDetail{
			Item:     item,
			Attempts: append([]StudioGenerationAttemptRecord(nil), detail.AttemptsByItem[item.ID]...),
			Designs:  append([]StudioMaterializedDesignRecord(nil), detail.DesignsByItem[item.ID]...),
		})
	}

	return &StudioBatchDetail{
		Batch: batch,
		Items: items,
	}
}

func projectStudioBatchRecord(batch *StudioBatchRecord, items []StudioBatchItemRecord) *StudioBatchRecord {
	if batch == nil {
		return nil
	}
	cloned := *batch
	if cloned.Status != StudioBatchStatusTasksCreated {
		cloned.Status = aggregateStudioBatchStatus(items)
	}
	return &cloned
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

func normalizeStudioBatchItemIDs(ids []string) []string {
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

func isStudioBatchItemRetryable(status StudioBatchItemStatus) bool {
	switch status {
	case StudioBatchItemStatusReviewReady, StudioBatchItemStatusFailed:
		return true
	default:
		return false
	}
}

func (s *taskStudioBatchService) refreshStudioBatchGenerationGraph(ctx context.Context, batchID string) error {
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

	_, existingErr := s.repo.GetStudioBatch(ctx, batchID)
	if existingErr != nil && !errors.Is(existingErr, gorm.ErrRecordNotFound) {
		return existingErr
	}

	now := s.currentTime().UTC()
	batch := buildStudioBatchRecordFromSessionDraft(session, now)
	items := expandStudioBatchItems(batch)
	for index := range items {
		items[index].CreatedAt = now.Add(time.Duration(index) * time.Second)
		items[index].UpdatedAt = items[index].CreatedAt
	}
	if errors.Is(existingErr, gorm.ErrRecordNotFound) {
		return s.repo.CreateStudioBatchGraph(ctx, batch, items, nil, nil)
	}
	return s.repo.ReplaceStudioBatchGenerationGraph(ctx, batch, items)
}

func (s *taskStudioBatchService) ensureStudioBatchGenerationGraphForResume(ctx context.Context, batchID string) error {
	if s.repo == nil {
		return fmt.Errorf("studio batch repository is not configured")
	}

	_, err := s.repo.GetStudioBatch(ctx, batchID)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return s.refreshStudioBatchGenerationGraph(ctx, batchID)
	default:
		return err
	}
}

func buildStudioBatchRecordFromSessionDraft(session *SheinStudioSession, now time.Time) *StudioBatchRecord {
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
	return batch
}

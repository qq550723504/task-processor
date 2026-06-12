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
	repo               StudioBatchRepository
	studioSessionRepo  studioBatchSeedSessionRepository
	generator          studioBatchGenerator
	createGenerateTask func(context.Context, *GenerateRequest) (*Task, error)
	getTask            func(context.Context, string) (*Task, error)
	currentTime        func() time.Time
}

func newTaskStudioBatchService(config taskStudioBatchServiceConfig) *taskStudioBatchService {
	return &taskStudioBatchService{
		repo:               config.repo,
		studioSessionRepo:  config.studioSessionRepo,
		generator:          config.generator,
		createGenerateTask: config.createGenerateTask,
		getTask:            config.getTask,
		currentTime:        time.Now,
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
	if shouldResumeStudioBatchTaskCreation(ctx, s.repo, normalizedBatchID) {
		result, err := s.resumeStudioBatchTaskCreation(ctx, normalizedBatchID)
		if err != nil {
			return nil, err
		}
		return &StudioBatchDetail{
			Batch:        result.Batch,
			Items:        result.Items,
			CreatedTasks: result.CreatedTasks,
			FailedTasks:  result.FailedTasks,
		}, nil
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
		fallbackDetail, syncRequired, syncErr := s.resolveStudioBatchDetailWithoutGraph(ctx, normalizedBatchID)
		if syncErr != nil {
			return nil, syncErr
		}
		if !syncRequired {
			return fallbackDetail, nil
		}
		if syncErr := s.ensureStudioBatchGenerationGraphForResume(ctx, normalizedBatchID); syncErr != nil {
			return nil, syncErr
		}
		detail, err = s.repo.GetStudioBatchDetail(ctx, normalizedBatchID)
	}
	if err != nil {
		return nil, err
	}

	draftUpdatedAt, createdTasks, failedTasks, draftErr := s.loadStudioBatchDraftState(ctx, normalizedBatchID)
	if draftErr != nil {
		return nil, draftErr
	}
	return projectStudioBatchDetail(detail, draftUpdatedAt, createdTasks, failedTasks), nil
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

	if err := s.syncStudioBatchRetryExecutionConfigFromDraft(ctx, normalizedBatchID); err != nil {
		return nil, err
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

func (s *taskStudioBatchService) syncStudioBatchRetryExecutionConfigFromDraft(ctx context.Context, batchID string) error {
	if s == nil || s.repo == nil || s.studioSessionRepo == nil {
		return nil
	}

	session, err := s.studioSessionRepo.GetSession(ctx, batchID)
	if err != nil {
		return err
	}
	if session == nil || !session.SavedAsBatch {
		return nil
	}

	batch, err := s.repo.GetStudioBatch(ctx, batchID)
	if err != nil {
		return err
	}
	if batch == nil {
		return nil
	}

	batch.Prompt = session.Prompt
	batch.StyleCount = session.StyleCount
	batch.VariationIntensity = session.VariationIntensity
	batch.ArtworkModel = session.ArtworkModel
	batch.SelectedSDSImages = append(SheinStudioSelectedSDSImageList(nil), session.SelectedSDSImages...)
	batch.TransparentBackground = session.TransparentBackground
	if storeID, convErr := strconv.ParseInt(strings.TrimSpace(session.SheinStoreID), 10, 64); convErr == nil {
		batch.SheinStoreID = storeID
	}
	batch.UpdatedAt = s.currentTime().UTC()
	return s.repo.UpdateStudioBatch(ctx, batch)
}

func (s *taskStudioBatchService) CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	if s.createGenerateTask == nil {
		return nil, fmt.Errorf("listing task creator is not configured")
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
	batchDetail, err := s.repo.GetStudioBatchDetail(ctx, normalizedBatchID)
	if err != nil {
		return nil, err
	}
	if batchDetail == nil || batchDetail.Batch == nil {
		return nil, gorm.ErrRecordNotFound
	}
	itemByID := make(map[string]StudioBatchItemRecord, len(batchDetail.Items))
	for _, item := range batchDetail.Items {
		itemByID[item.ID] = item
	}

	var session *SheinStudioSession
	if s.studioSessionRepo != nil {
		session, err = s.studioSessionRepo.GetSession(ctx, normalizedBatchID)
		if err != nil {
			return nil, err
		}
	}
	if session == nil {
		return nil, ErrStudioSessionNotFound
	}

	sessionDesignsByID := map[string]SheinStudioDesign{}
	if designSource, ok := s.studioSessionRepo.(interface {
		ListSessionDesigns(context.Context, string) ([]SheinStudioDesign, error)
	}); ok {
		sessionDesigns, listErr := designSource.ListSessionDesigns(ctx, normalizedBatchID)
		if listErr != nil {
			return nil, listErr
		}
		for _, design := range sessionDesigns {
			sessionDesignsByID[strings.TrimSpace(design.ID)] = design
		}
	}

	createdTasks := make([]SheinStudioCreatedTask, 0, len(designs))
	failedTasks := make([]SheinStudioFailedTask, 0)
	for _, design := range designs {
		if design.ReviewStatus != StudioMaterializedDesignReviewStatusApproved {
			return nil, NewStudioBatchActionValidationError(fmt.Sprintf("design %s is not approved", design.ID))
		}
		item, ok := itemByID[design.ItemID]
		if !ok {
			return nil, gorm.ErrRecordNotFound
		}
		selections := resolveStudioBatchItemSelections(batchDetail.Batch, item)
		if len(selections) == 0 {
			selections = []SheinStudioGroupedSelection{{
				SelectionID:  selectionIDForStudioSelection(SheinStudioSelection(session.Selection)),
				Selection:    SheinStudioSelection(session.Selection),
				Eligible:     true,
				SheinStoreID: strings.TrimSpace(session.SheinStoreID),
			}}
		}
		for _, grouped := range selections {
			title := firstNonEmpty(
				strings.TrimSpace(grouped.Selection.VariantLabel),
				strings.TrimSpace(grouped.Selection.ProductName),
				strings.TrimSpace(design.TargetGroupLabel),
				strings.TrimSpace(design.ID),
			)
			if existing, ok := s.findExistingStudioBatchTask(ctx, session.CreatedTasks, design, grouped, title); ok {
				createdTasks = append(createdTasks, existing)
				continue
			}
			task, createErr := s.createGenerateTask(
				ctx,
				buildStudioBatchTaskGenerateRequest(session, grouped, design, sessionDesignsByID[design.ID]),
			)
			if createErr != nil {
				failedTasks = append(failedTasks, SheinStudioFailedTask{
					DesignID: design.ID,
					Title:    title,
					Message:  createErr.Error(),
				})
				continue
			}
			createdTasks = append(createdTasks, SheinStudioCreatedTask{
				ID:       task.ID,
				Title:    title,
				DesignID: design.ID,
			})
		}
	}

	if sessionUpdater, ok := s.studioSessionRepo.(interface {
		UpdateSession(context.Context, *SheinStudioSession) error
	}); ok {
		session.CreatedTasks = mergeStudioCreatedTasks(session.CreatedTasks, createdTasks)
		session.CreatedTaskIDs = buildCreatedTaskIDs(session.CreatedTasks)
		session.FailedTasks = append(SheinStudioFailedTaskList(nil), failedTasks...)
		session.UpdatedAt = s.currentTime().UTC()
		if err := sessionUpdater.UpdateSession(ctx, session); err != nil {
			return nil, err
		}
	}
	if len(createdTasks) > 0 {
		batch, err := s.repo.GetStudioBatch(ctx, normalizedBatchID)
		if err != nil {
			return nil, err
		}
		batch.Status = StudioBatchStatusTasksCreated
		batch.UpdatedAt = s.currentTime().UTC()
		if err := s.repo.UpdateStudioBatch(ctx, batch); err != nil {
			return nil, err
		}
	}

	detail, err := s.GetStudioBatchDetail(ctx, normalizedBatchID)
	if err != nil {
		return nil, err
	}
	return &CreateStudioBatchTasksResult{
		Batch:        detail.Batch,
		Items:        detail.Items,
		CreatedTasks: createdTasks,
		FailedTasks:  failedTasks,
	}, nil
}

func (s *taskStudioBatchService) PrepareCreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	designIDs, session, _, err := s.prepareStudioBatchTaskCreation(ctx, normalizedBatchID, req)
	if err != nil {
		return nil, err
	}
	session.PendingTaskDesignIDs = append(SheinStudioStringList(nil), designIDs...)
	session.FailedTasks = nil
	session.Status = SheinStudioSessionStatusTasksCreating
	session.UpdatedAt = s.currentTime().UTC()
	if sessionUpdater, ok := s.studioSessionRepo.(interface {
		UpdateSession(context.Context, *SheinStudioSession) error
	}); ok {
		if err := sessionUpdater.UpdateSession(ctx, session); err != nil {
			return nil, err
		}
	}
	batch, err := s.repo.GetStudioBatch(ctx, normalizedBatchID)
	if err != nil {
		return nil, err
	}
	batch.Status = StudioBatchStatusTasksCreating
	batch.UpdatedAt = s.currentTime().UTC()
	if err := s.repo.UpdateStudioBatch(ctx, batch); err != nil {
		return nil, err
	}
	currentDetail, err := s.GetStudioBatchDetail(ctx, normalizedBatchID)
	if err != nil {
		return nil, err
	}
	return &CreateStudioBatchTasksResult{
		Batch:        currentDetail.Batch,
		Items:        currentDetail.Items,
		CreatedTasks: currentDetail.CreatedTasks,
		FailedTasks:  currentDetail.FailedTasks,
	}, nil
}

func (s *taskStudioBatchService) resumeStudioBatchTaskCreation(ctx context.Context, batchID string) (*CreateStudioBatchTasksResult, error) {
	session, err := s.loadStudioBatchTaskSession(ctx, batchID)
	if err != nil {
		return nil, err
	}
	designIDs := normalizeStudioBatchDesignIDs([]string(session.PendingTaskDesignIDs))
	if len(designIDs) == 0 {
		detail, detailErr := s.GetStudioBatchDetail(ctx, batchID)
		if detailErr != nil {
			return nil, detailErr
		}
		return &CreateStudioBatchTasksResult{
			Batch:        detail.Batch,
			Items:        detail.Items,
			CreatedTasks: detail.CreatedTasks,
			FailedTasks:  detail.FailedTasks,
		}, nil
	}
	result, err := s.CreateStudioBatchTasks(ctx, batchID, &CreateStudioBatchTasksRequest{DesignIDs: designIDs})
	if err != nil {
		return nil, err
	}
	if sessionUpdater, ok := s.studioSessionRepo.(interface {
		UpdateSession(context.Context, *SheinStudioSession) error
	}); ok {
		session.PendingTaskDesignIDs = nil
		session.CreatedTasks = append(SheinStudioCreatedTaskList(nil), result.CreatedTasks...)
		session.CreatedTaskIDs = buildCreatedTaskIDs(result.CreatedTasks)
		session.FailedTasks = append(SheinStudioFailedTaskList(nil), result.FailedTasks...)
		session.Status = SheinStudioSessionStatusTasksCreated
		session.UpdatedAt = s.currentTime().UTC()
		if err := sessionUpdater.UpdateSession(ctx, session); err != nil {
			return nil, err
		}
	}
	batch, err := s.repo.GetStudioBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	batch.Status = StudioBatchStatusTasksCreated
	batch.UpdatedAt = s.currentTime().UTC()
	if err := s.repo.UpdateStudioBatch(ctx, batch); err != nil {
		return nil, err
	}
	detail, err := s.GetStudioBatchDetail(ctx, batchID)
	if err != nil {
		return nil, err
	}
	return &CreateStudioBatchTasksResult{
		Batch:        detail.Batch,
		Items:        detail.Items,
		CreatedTasks: detail.CreatedTasks,
		FailedTasks:  detail.FailedTasks,
	}, nil
}

func (s *taskStudioBatchService) prepareStudioBatchTaskCreation(
	ctx context.Context,
	batchID string,
	req *CreateStudioBatchTasksRequest,
) ([]string, *SheinStudioSession, *StudioBatchDetailGraph, error) {
	designIDs := normalizeStudioBatchDesignIDs(nil)
	if req != nil {
		designIDs = normalizeStudioBatchDesignIDs(req.DesignIDs)
	}
	if len(designIDs) == 0 {
		return nil, nil, nil, NewStudioBatchActionValidationError("design_ids is required")
	}
	designs, err := s.repo.ListStudioMaterializedDesignsByIDs(ctx, batchID, designIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(designs) != len(designIDs) {
		return nil, nil, nil, gorm.ErrRecordNotFound
	}
	batchDetail, err := s.repo.GetStudioBatchDetail(ctx, batchID)
	if err != nil {
		return nil, nil, nil, err
	}
	if batchDetail == nil || batchDetail.Batch == nil {
		return nil, nil, nil, gorm.ErrRecordNotFound
	}
	for _, design := range designs {
		if design.ReviewStatus != StudioMaterializedDesignReviewStatusApproved {
			return nil, nil, nil, NewStudioBatchActionValidationError(fmt.Sprintf("design %s is not approved", design.ID))
		}
	}
	session, err := s.loadStudioBatchTaskSession(ctx, batchID)
	if err != nil {
		return nil, nil, nil, err
	}
	return designIDs, session, batchDetail, nil
}

func (s *taskStudioBatchService) loadStudioBatchTaskSession(ctx context.Context, batchID string) (*SheinStudioSession, error) {
	if s.studioSessionRepo == nil {
		return nil, ErrStudioSessionNotFound
	}
	session, err := s.studioSessionRepo.GetSession(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrStudioSessionNotFound
	}
	return session, nil
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

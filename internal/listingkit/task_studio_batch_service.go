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

func (s *taskStudioBatchService) resolveStudioBatchDetailWithoutGraph(ctx context.Context, batchID string) (*StudioBatchDetail, bool, error) {
	if s.studioSessionRepo == nil {
		return nil, false, gorm.ErrRecordNotFound
	}
	session, err := s.studioSessionRepo.GetSession(ctx, batchID)
	if err != nil {
		return nil, false, err
	}
	if session == nil || !session.SavedAsBatch {
		return nil, false, ErrStudioSessionNotFound
	}
	if shouldSyncStudioBatchGraphOnRead(session) {
		return nil, true, nil
	}
	return buildStudioBatchDraftOnlyDetail(session), false, nil
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

func shouldResumeStudioBatchTaskCreation(ctx context.Context, repo StudioBatchRepository, batchID string) bool {
	if repo == nil {
		return false
	}
	batch, err := repo.GetStudioBatch(ctx, batchID)
	if err != nil || batch == nil {
		return false
	}
	return batch.Status == StudioBatchStatusTasksCreating
}

func (s *taskStudioBatchService) findExistingStudioBatchTask(
	ctx context.Context,
	recorded SheinStudioCreatedTaskList,
	design StudioMaterializedDesignRecord,
	grouped SheinStudioGroupedSelection,
	fallbackTitle string,
) (SheinStudioCreatedTask, bool) {
	if s == nil || s.getTask == nil || len(recorded) == 0 {
		return SheinStudioCreatedTask{}, false
	}
	designID := strings.TrimSpace(design.ID)
	for _, created := range recorded {
		if strings.TrimSpace(created.DesignID) != designID || strings.TrimSpace(created.ID) == "" {
			continue
		}
		task, err := s.getTask(ctx, created.ID)
		if err != nil || task == nil || task.Status == TaskStatusFailed {
			continue
		}
		if !studioBatchTaskMatchesSelection(task, design, grouped.Selection) {
			continue
		}
		if strings.TrimSpace(created.Title) == "" {
			created.Title = fallbackTitle
		}
		return created, true
	}
	return SheinStudioCreatedTask{}, false
}

func studioBatchTaskMatchesSelection(
	task *Task,
	design StudioMaterializedDesignRecord,
	selection SheinStudioSelection,
) bool {
	if task == nil || task.Request == nil || task.Request.Options == nil {
		return false
	}
	studio := task.Request.Options.SheinStudio
	sds := task.Request.Options.SDS
	if studio == nil || sds == nil {
		return false
	}
	if strings.TrimSpace(studio.StyleID) != buildStudioBatchTaskStyleID(design.ID) {
		return false
	}
	if len(task.Request.ImageURLs) == 0 || strings.TrimSpace(task.Request.ImageURLs[0]) != strings.TrimSpace(design.ImageURL) {
		return false
	}
	return sds.VariantID == selection.VariantID &&
		sds.ParentProductID == selection.ParentProductID &&
		sds.PrototypeGroupID == selection.PrototypeGroupID &&
		strings.TrimSpace(sds.LayerID) == strings.TrimSpace(selection.LayerID)
}

func mergeStudioCreatedTasks(
	existing SheinStudioCreatedTaskList,
	created []SheinStudioCreatedTask,
) SheinStudioCreatedTaskList {
	if len(existing) == 0 && len(created) == 0 {
		return nil
	}
	merged := make(SheinStudioCreatedTaskList, 0, len(existing)+len(created))
	seen := make(map[string]struct{}, len(existing)+len(created))
	appendIfMissing := func(task SheinStudioCreatedTask) {
		id := strings.TrimSpace(task.ID)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		merged = append(merged, task)
	}
	for _, task := range existing {
		appendIfMissing(task)
	}
	for _, task := range created {
		appendIfMissing(task)
	}
	return merged
}

func projectStudioBatchDetail(
	detail *StudioBatchDetailGraph,
	draftUpdatedAt *time.Time,
	createdTasks []SheinStudioCreatedTask,
	failedTasks []SheinStudioFailedTask,
) *StudioBatchDetail {
	if detail == nil {
		return &StudioBatchDetail{}
	}

	batch := projectStudioBatchRecord(detail.Batch, detail.Items, draftUpdatedAt)
	items := make([]StudioBatchItemDetail, 0, len(detail.Items))
	for _, item := range detail.Items {
		items = append(items, StudioBatchItemDetail{
			Item:     item,
			Attempts: append([]StudioGenerationAttemptRecord(nil), detail.AttemptsByItem[item.ID]...),
			Designs:  append([]StudioMaterializedDesignRecord(nil), detail.DesignsByItem[item.ID]...),
		})
	}

	return &StudioBatchDetail{
		Batch:        batch,
		Items:        items,
		CreatedTasks: append([]SheinStudioCreatedTask(nil), createdTasks...),
		FailedTasks:  append([]SheinStudioFailedTask(nil), failedTasks...),
	}
}

func buildStudioBatchTaskGenerateRequest(
	session *SheinStudioSession,
	groupedSelection SheinStudioGroupedSelection,
	design StudioMaterializedDesignRecord,
	sessionDesign SheinStudioDesign,
) *GenerateRequest {
	if session == nil {
		return &GenerateRequest{}
	}
	selection := groupedSelection.Selection
	storeID := parseStudioBatchTaskStoreID(groupedSelection.SheinStoreID)
	if storeID <= 0 {
		storeID = parseStudioBatchTaskStoreID(session.SheinStoreID)
	}

	styleID := buildStudioBatchTaskStyleID(design.ID)
	styleName := firstNonEmpty(
		strings.TrimSpace(design.TargetGroupLabel),
		strings.TrimSpace(selection.ProductName),
		strings.TrimSpace(design.ID),
	)
	req := &GenerateRequest{
		TenantID:     strings.TrimSpace(session.TenantID),
		UserID:       strings.TrimSpace(session.UserID),
		Text:         strings.TrimSpace(session.Prompt),
		ImageURLs:    []string{strings.TrimSpace(design.ImageURL)},
		Platforms:    []string{"shein"},
		SheinStoreID: storeID,
		Options: &GenerateOptions{
			ImageStrategy: strings.TrimSpace(session.ImageStrategy),
			ProcessImages: false,
			SheinStudio: &SheinStudioOptions{
				StyleID:                 styleID,
				StyleName:               styleName,
				SourceDesignURLs:        []string{strings.TrimSpace(design.ImageURL)},
				ProductImageURLs:        append([]string(nil), sessionDesign.ProductImageURLs...),
				SelectedSDSImages:       toGenerateRequestSelectedSDSImages(session.SelectedSDSImages),
				SizeReferenceImageURLs:  append([]string(nil), selection.SizeReferenceImageURLs...),
				RenderSizeImagesWithSDS: session.RenderSizeImagesWithSDS,
			},
			SDS: buildStudioBatchTaskSDSOptions(selection, styleID, styleName),
		},
	}
	return req
}

func buildStudioBatchTaskSDSOptions(
	selection SheinStudioSelection,
	styleID string,
	styleName string,
) *SDSSyncOptions {
	return &SDSSyncOptions{
		VariantID:        selection.VariantID,
		ParentProductID:  selection.ParentProductID,
		PrototypeGroupID: selection.PrototypeGroupID,
		LayerID:          selection.LayerID,
		DesignType:       "material", // Default design type
		ProductName:      selection.ProductName,
		BlankDesignURL:   selection.BlankDesignURL,
		TemplateImageURL: selection.TemplateImageURL,
		MaskImageURL:     selection.MaskImageURL,
		PrintableWidth:   selection.PrintableWidth,
		PrintableHeight:  selection.PrintableHeight,
		MockupImageURLs:  append([]string(nil), selection.MockupImageURLs...),
		StyleID:          styleID,
		StyleName:        styleName,
		Variants:         buildStudioBatchTaskVariantOptions(selection.Variants),
	}
}

func buildStudioBatchTaskVariantOptions(
	variants []SheinStudioSelectionVariant,
) []SDSSyncVariantOption {
	if len(variants) == 0 {
		return nil
	}
	result := make([]SDSSyncVariantOption, 0, len(variants))
	for _, variant := range variants {
		result = append(result, SDSSyncVariantOption{
			VariantID:              variant.VariantID,
			VariantSKU:             variant.VariantSKU,
			Size:                   variant.Size,
			Color:                  variant.Color,
			Price:                  variant.Price,
			Weight:                 variant.Weight,
			BoxLength:              variant.BoxLength,
			BoxWidth:               variant.BoxWidth,
			BoxHeight:              variant.BoxHeight,
			ProductionCycle:        variant.ProductionCycle,
			PrototypeGroupID:       variant.PrototypeGroupID,
			LayerID:                variant.LayerID,
			TemplateImageURL:       variant.TemplateImageURL,
			MaskImageURL:           variant.MaskImageURL,
			BlankDesignURL:         variant.BlankDesignURL,
			MockupImageURL:         variant.MockupImageURL,
			MockupImageURLs:        append([]string(nil), variant.MockupImageURLs...),
			SizeReferenceImageURLs: append([]string(nil), variant.SizeReferenceImageURLs...),
		})
	}
	return result
}

func toGenerateRequestSelectedSDSImages(
	input SheinStudioSelectedSDSImageList,
) []SheinStudioSelectedSDSImage {
	if len(input) == 0 {
		return nil
	}
	result := make([]SheinStudioSelectedSDSImage, 0, len(input))
	for _, item := range input {
		result = append(result, SheinStudioSelectedSDSImage{
			ImageURL:   item.ImageURL,
			VariantSKU: item.VariantSKU,
			Color:      item.Color,
		})
	}
	return result
}

func parseStudioBatchTaskStoreID(raw string) int64 {
	storeID, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0
	}
	return storeID
}

func buildStudioBatchTaskStyleID(designID string) string {
	compact := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r - ('a' - 'A')
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return -1
		}
	}, strings.TrimSpace(designID))
	if len(compact) > 8 {
		return compact[:8]
	}
	if compact == "" {
		return "STYLE001"
	}
	return compact
}

func projectStudioBatchRecord(batch *StudioBatchRecord, items []StudioBatchItemRecord, draftUpdatedAt *time.Time) *StudioBatchRecord {
	if batch == nil {
		return nil
	}
	cloned := *batch
	if cloned.Status != StudioBatchStatusTasksCreated {
		cloned.Status = aggregateStudioBatchStatus(items)
	}
	cloned.DraftUpdatedAt = draftUpdatedAt
	return &cloned
}

func (s *taskStudioBatchService) loadStudioBatchDraftState(ctx context.Context, batchID string) (*time.Time, []SheinStudioCreatedTask, []SheinStudioFailedTask, error) {
	if s.studioSessionRepo == nil {
		return nil, nil, nil, nil
	}
	session, err := s.studioSessionRepo.GetSession(ctx, batchID)
	switch {
	case err == nil:
		if session == nil || !session.SavedAsBatch {
			return nil, nil, nil, nil
		}
		updatedAt := session.UpdatedAt.UTC()
		return &updatedAt, append([]SheinStudioCreatedTask(nil), session.CreatedTasks...), append([]SheinStudioFailedTask(nil), session.FailedTasks...), nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, nil, nil, nil
	default:
		return nil, nil, nil, err
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

func shouldSyncStudioBatchGraphOnRead(session *SheinStudioSession) bool {
	if session == nil {
		return false
	}
	if session.Status == SheinStudioSessionStatusGenerating {
		return true
	}
	if strings.TrimSpace(session.GenerationJobID) != "" {
		return true
	}
	return len(session.GenerationJobs) > 0
}

func buildStudioBatchDraftOnlyDetail(session *SheinStudioSession) *StudioBatchDetail {
	if session == nil {
		return &StudioBatchDetail{}
	}
	batch := buildStudioBatchRecordFromSessionDraft(session, session.UpdatedAt.UTC())
	batch.Status = StudioBatchStatusDraft
	updatedAt := session.UpdatedAt.UTC()
	batch.DraftUpdatedAt = &updatedAt
	return &StudioBatchDetail{
		Batch:        batch,
		Items:        []StudioBatchItemDetail{},
		CreatedTasks: append([]SheinStudioCreatedTask(nil), session.CreatedTasks...),
		FailedTasks:  append([]SheinStudioFailedTask(nil), session.FailedTasks...),
	}
}

package listingkit

import (
	"context"
	"fmt"
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
	batchRunner        *listingStudioBatchGenerationRunner
	detailRunner       *listingStudioBatchDetailRunner
	reviewRunner       *listingStudioBatchReviewRunner
	retryRunner        *listingStudioBatchRetryPrepareRunner
	taskPrepareRunner  *listingStudioBatchTaskPrepareRunner
	taskResumeRunner   *listingStudioBatchTaskResumeRunner
}

func newTaskStudioBatchService(config taskStudioBatchServiceConfig) *taskStudioBatchService {
	service := &taskStudioBatchService{
		repo:               config.repo,
		studioSessionRepo:  config.studioSessionRepo,
		generator:          config.generator,
		createGenerateTask: config.createGenerateTask,
		getTask:            config.getTask,
		currentTime:        time.Now,
		batchRunner:        config.batchRunner,
		detailRunner:       config.detailRunner,
		reviewRunner:       config.reviewRunner,
		retryRunner:        config.retryRunner,
		taskPrepareRunner:  config.taskPrepareRunner,
		taskResumeRunner:   config.taskResumeRunner,
	}
	service.ensureBatchRunner()
	service.ensureDetailRunner()
	service.ensureReviewRunner()
	service.ensureRetryRunner()
	service.ensureTaskPrepareRunner()
	service.ensureTaskResumeRunner()
	return service
}

func (s *taskStudioBatchService) StartStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	s.ensureBatchRunner()
	if s.batchRunner == nil {
		return nil, fmt.Errorf("studio batch generation runner is not configured")
	}
	return s.batchRunner.StartGeneration(ctx, strings.TrimSpace(batchID))
}

func (s *taskStudioBatchService) PrepareStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	s.ensureBatchRunner()
	if s.batchRunner == nil {
		return nil, fmt.Errorf("studio batch generation runner is not configured")
	}
	return s.batchRunner.PrepareGeneration(ctx, strings.TrimSpace(batchID))
}

func (s *taskStudioBatchService) ResumeStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	s.ensureBatchRunner()
	if s.batchRunner == nil {
		return nil, fmt.Errorf("studio batch generation runner is not configured")
	}
	return s.batchRunner.ResumeGeneration(ctx, strings.TrimSpace(batchID))
}

func (s *taskStudioBatchService) continueStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	if s.generator == nil {
		return nil, fmt.Errorf("studio batch generator is not configured")
	}
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
	s.ensureDetailRunner()
	if s.detailRunner == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	return s.detailRunner.GetDetail(ctx, normalizedBatchID)
}

func (s *taskStudioBatchService) ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error) {
	s.ensureReviewRunner()
	if s.reviewRunner == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	approvedIDs := normalizeStudioBatchDesignIDs(nil)
	if req != nil {
		approvedIDs = normalizeStudioBatchDesignIDs(req.DesignIDs)
	}
	return s.reviewRunner.ApproveDesigns(ctx, normalizedBatchID, approvedIDs)
}

func (s *taskStudioBatchService) RetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {
	normalizedBatchID := strings.TrimSpace(batchID)
	itemIDs := normalizeStudioBatchItemIDs(nil)
	if req != nil {
		itemIDs = normalizeStudioBatchItemIDs(req.ItemIDs)
	}
	s.ensureBatchRunner()
	if s.batchRunner == nil {
		return nil, fmt.Errorf("studio batch generation runner is not configured")
	}
	return s.batchRunner.RetryItems(ctx, normalizedBatchID, itemIDs)
}

func (s *taskStudioBatchService) PrepareRetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	if s.generator == nil {
		return nil, fmt.Errorf("studio batch generator is not configured")
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	itemIDs := normalizeStudioBatchItemIDs(nil)
	if req != nil {
		itemIDs = normalizeStudioBatchItemIDs(req.ItemIDs)
	}
	if err := s.syncStudioBatchRetryExecutionConfigFromDraft(ctx, normalizedBatchID); err != nil {
		return nil, err
	}
	s.ensureRetryRunner()
	if s.retryRunner == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	return s.retryRunner.PrepareRetryItems(ctx, normalizedBatchID, itemIDs)
}

func (s *taskStudioBatchService) ensureDetailRunner() {
	if s == nil || s.detailRunner != nil {
		return
	}
	s.detailRunner = newListingStudioBatchDetailService(s.repo, s.studioSessionRepo, s.ensureStudioBatchGenerationGraphForResume)
}

func (s *taskStudioBatchService) ensureBatchRunner() {
	if s == nil || s.batchRunner != nil {
		return
	}
	s.batchRunner = newListingStudioBatchGenerationService(s)
}

func (s *taskStudioBatchService) ensureReviewRunner() {
	if s == nil || s.reviewRunner != nil {
		return
	}
	s.reviewRunner = newListingStudioBatchReviewService(s.repo, s.GetStudioBatchDetail, s.currentTime)
}

func (s *taskStudioBatchService) ensureRetryRunner() {
	if s == nil || s.retryRunner != nil {
		return
	}
	s.retryRunner = newListingStudioBatchRetryPrepareService(s.repo, s.GetStudioBatchDetail, s.resetStudioBatchRetryItems)
}

func (s *taskStudioBatchService) ensureTaskPrepareRunner() {
	if s == nil || s.taskPrepareRunner != nil {
		return
	}
	var updateSession func(context.Context, *SheinStudioSession) error
	if sessionUpdater, ok := s.studioSessionRepo.(interface {
		UpdateSession(context.Context, *SheinStudioSession) error
	}); ok {
		updateSession = sessionUpdater.UpdateSession
	}
	var updateBatch func(context.Context, *StudioBatchRecord) error
	if s.repo != nil {
		updateBatch = s.repo.UpdateStudioBatch
	}
	s.taskPrepareRunner = newListingStudioBatchTaskPrepareService(
		updateSession,
		updateBatch,
		s.loadStudioBatchTaskPreparationResult,
		s.currentTime,
	)
}

func (s *taskStudioBatchService) ensureTaskResumeRunner() {
	if s == nil || s.taskResumeRunner != nil {
		return
	}
	var updateSession func(context.Context, *SheinStudioSession) error
	if sessionUpdater, ok := s.studioSessionRepo.(interface {
		UpdateSession(context.Context, *SheinStudioSession) error
	}); ok {
		updateSession = sessionUpdater.UpdateSession
	}
	var updateBatch func(context.Context, *StudioBatchRecord) error
	if s.repo != nil {
		updateBatch = s.repo.UpdateStudioBatch
	}
	s.taskResumeRunner = newListingStudioBatchTaskResumeService(
		updateSession,
		updateBatch,
		s.loadStudioBatchTaskPreparationResult,
		s.currentTime,
	)
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

	sessionDesignsByID, err := s.loadStudioBatchSessionDesigns(ctx, normalizedBatchID)
	if err != nil {
		return nil, err
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
	batch, err := s.repo.GetStudioBatch(ctx, normalizedBatchID)
	if err != nil {
		return nil, err
	}
	s.ensureTaskPrepareRunner()
	if s.taskPrepareRunner == nil {
		return nil, fmt.Errorf("studio batch task prepare runner is not configured")
	}
	return s.taskPrepareRunner.PrepareTaskCreation(ctx, normalizedBatchID, listingStudioBatchTaskPrepareState{
		Session:   session,
		Batch:     batch,
		DesignIDs: designIDs,
	})
}

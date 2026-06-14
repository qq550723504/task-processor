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
	detailRunner       *listingStudioBatchDetailRunner
	reviewRunner       *listingStudioBatchReviewRunner
}

func newTaskStudioBatchService(config taskStudioBatchServiceConfig) *taskStudioBatchService {
	service := &taskStudioBatchService{
		repo:               config.repo,
		studioSessionRepo:  config.studioSessionRepo,
		generator:          config.generator,
		createGenerateTask: config.createGenerateTask,
		getTask:            config.getTask,
		currentTime:        time.Now,
		detailRunner:       config.detailRunner,
		reviewRunner:       config.reviewRunner,
	}
	service.ensureDetailRunner()
	service.ensureReviewRunner()
	return service
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

	if err := s.syncStudioBatchRetryExecutionConfigFromDraft(ctx, normalizedBatchID); err != nil {
		return nil, err
	}

	itemsToRetry, err := selectStudioBatchRetryItems(detail, itemIDs)
	if err != nil {
		return nil, err
	}
	if err := s.resetStudioBatchRetryItems(ctx, itemsToRetry); err != nil {
		return nil, err
	}

	return s.GetStudioBatchDetail(ctx, normalizedBatchID)
}

func (s *taskStudioBatchService) ensureDetailRunner() {
	if s == nil || s.detailRunner != nil {
		return
	}
	s.detailRunner = newListingStudioBatchDetailService(s.repo, s.studioSessionRepo, s.ensureStudioBatchGenerationGraphForResume)
}

func (s *taskStudioBatchService) ensureReviewRunner() {
	if s == nil || s.reviewRunner != nil {
		return
	}
	s.reviewRunner = newListingStudioBatchReviewService(s.repo, s.GetStudioBatchDetail, s.currentTime)
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

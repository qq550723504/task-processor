package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"
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
	taskCreationRunner *listingStudioBatchTaskCreationRunner
	taskExecuteRunner  *listingStudioBatchTaskExecuteRunner
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
		taskCreationRunner: config.taskCreationRunner,
		taskExecuteRunner:  config.taskExecuteRunner,
		taskPrepareRunner:  config.taskPrepareRunner,
		taskResumeRunner:   config.taskResumeRunner,
	}
	service.ensureBatchRunner()
	service.ensureDetailRunner()
	service.ensureReviewRunner()
	service.ensureRetryRunner()
	service.ensureTaskCreationRunner()
	service.ensureTaskExecuteRunner()
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

func (s *taskStudioBatchService) ensureTaskCreationRunner() {
	if s == nil || s.taskCreationRunner != nil {
		return
	}
	s.taskCreationRunner = newListingStudioBatchTaskCreationService(s)
}

func (s *taskStudioBatchService) ensureTaskExecuteRunner() {
	if s == nil || s.taskExecuteRunner != nil {
		return
	}
	s.taskExecuteRunner = newListingStudioBatchTaskExecuteService(s)
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
	normalizedBatchID := strings.TrimSpace(batchID)
	designIDs := normalizeStudioBatchDesignIDs(nil)
	if req != nil {
		designIDs = normalizeStudioBatchDesignIDs(req.DesignIDs)
	}
	s.ensureTaskExecuteRunner()
	if s.taskExecuteRunner == nil {
		return nil, fmt.Errorf("studio batch task execute service is not configured")
	}
	return s.taskExecuteRunner.Execute(ctx, normalizedBatchID, designIDs)
}

func (s *taskStudioBatchService) PrepareCreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	designIDs := normalizeStudioBatchDesignIDs(nil)
	if req != nil {
		designIDs = normalizeStudioBatchDesignIDs(req.DesignIDs)
	}
	designIDs, session, batchDetail, err := s.prepareStudioBatchTaskCreation(ctx, normalizedBatchID, &CreateStudioBatchTasksRequest{
		DesignIDs: designIDs,
	})
	if err != nil {
		return nil, err
	}
	s.ensureTaskPrepareRunner()
	if s.taskPrepareRunner == nil {
		return nil, fmt.Errorf("studio batch task prepare runner is not configured")
	}
	return s.taskPrepareRunner.PrepareTaskCreation(ctx, normalizedBatchID, listingStudioBatchTaskPrepareState{
		Session:   session,
		Batch:     batchDetail.Batch,
		DesignIDs: designIDs,
	})
}

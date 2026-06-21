package listingkit

import (
	"context"
	"fmt"
	"time"
)

type taskStudioBatchService struct {
	repo               StudioBatchRepository
	batchRunRepo       StudioBatchRunRepository
	batchTaskLinkRepo  StudioBatchTaskLinkRepository
	studioSessionRepo  studioBatchSeedSessionRepository
	baselineChecker    StudioBatchBaselineReadinessChecker
	storeValidator     StudioBatchStoreValidator
	generator          studioBatchGenerator
	createGenerateTask func(context.Context, *GenerateRequest) (*Task, error)
	getTask            func(context.Context, string) (*Task, error)
	currentTime        func() time.Time
	serviceRunner      *listingStudioBatchServiceRunner
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
		batchRunRepo:       config.batchRunRepo,
		batchTaskLinkRepo:  config.batchTaskLinkRepo,
		studioSessionRepo:  config.studioSessionRepo,
		baselineChecker:    config.baselineChecker,
		storeValidator:     config.storeValidator,
		generator:          config.generator,
		createGenerateTask: config.createGenerateTask,
		getTask:            config.getTask,
		currentTime:        time.Now,
		serviceRunner:      config.serviceRunner,
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
	service.ensureServiceRunner()
	return service
}

func (s *taskStudioBatchService) StartStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	s.ensureServiceRunner()
	if s.serviceRunner == nil {
		return nil, fmt.Errorf("studio batch service is not configured")
	}
	return s.serviceRunner.StartGeneration(ctx, batchID)
}

func (s *taskStudioBatchService) PrepareStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	s.ensureServiceRunner()
	if s.serviceRunner == nil {
		return nil, fmt.Errorf("studio batch service is not configured")
	}
	return s.serviceRunner.PrepareGeneration(ctx, batchID)
}

func (s *taskStudioBatchService) ResumeStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	s.ensureServiceRunner()
	if s.serviceRunner == nil {
		return nil, fmt.Errorf("studio batch service is not configured")
	}
	return s.serviceRunner.ResumeGeneration(ctx, batchID)
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
	if err := s.requeueFailedStudioBatchItemsForContinue(ctx, batchID); err != nil {
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

func (s *taskStudioBatchService) requeueFailedStudioBatchItemsForContinue(ctx context.Context, batchID string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	detail, err := s.repo.GetStudioBatchDetail(ctx, batchID)
	if err != nil {
		return err
	}
	if detail == nil || len(detail.Items) == 0 {
		return nil
	}

	failedItems := make([]StudioBatchItemRecord, 0)
	for _, item := range detail.Items {
		if item.Status != StudioBatchItemStatusFailed {
			continue
		}
		failedItems = append(failedItems, item)
	}
	if len(failedItems) == 0 {
		return nil
	}
	return s.resetStudioBatchRetryItems(ctx, failedItems)
}

func (s *taskStudioBatchService) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	s.ensureServiceRunner()
	if s.serviceRunner == nil {
		return nil, fmt.Errorf("studio batch service is not configured")
	}
	return s.serviceRunner.GetDetail(ctx, batchID)
}

func (s *taskStudioBatchService) ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error) {
	s.ensureServiceRunner()
	if s.serviceRunner == nil {
		return nil, fmt.Errorf("studio batch service is not configured")
	}
	return s.serviceRunner.ApproveDesigns(ctx, batchID, req)
}

func (s *taskStudioBatchService) RetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {
	s.ensureServiceRunner()
	if s.serviceRunner == nil {
		return nil, fmt.Errorf("studio batch service is not configured")
	}
	return s.serviceRunner.RetryItems(ctx, batchID, req)
}

func (s *taskStudioBatchService) PrepareRetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {
	s.ensureServiceRunner()
	if s.serviceRunner == nil {
		return nil, fmt.Errorf("studio batch service is not configured")
	}
	return s.serviceRunner.PrepareRetryItems(ctx, batchID, req)
}

func (s *taskStudioBatchService) CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {
	s.ensureServiceRunner()
	if s.serviceRunner == nil {
		return nil, fmt.Errorf("studio batch service is not configured")
	}
	if req != nil && req.AllowPartialWhileGenerating {
		ctx = withStudioBatchPartialTaskCreationAllowed(ctx)
	}
	return s.serviceRunner.CreateTasks(ctx, batchID, req)
}

func (s *taskStudioBatchService) PrepareCreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {
	s.ensureServiceRunner()
	if s.serviceRunner == nil {
		return nil, fmt.Errorf("studio batch service is not configured")
	}
	if req != nil && req.AllowPartialWhileGenerating {
		ctx = withStudioBatchPartialTaskCreationAllowed(ctx)
	}
	return s.serviceRunner.PrepareCreateTasks(ctx, batchID, req)
}

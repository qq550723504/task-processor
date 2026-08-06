package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type taskStudioBatchService struct {
	repo                     StudioBatchRepository
	batchRunRepo             StudioBatchRunRepository
	batchTaskLinkRepo        StudioBatchTaskLinkRepository
	studioSessionRepo        studioBatchSeedSessionRepository
	baselineChecker          StudioBatchBaselineReadinessChecker
	sdsProductDetailProvider SDSBaselineRemoteProvider
	storeValidator           StudioBatchStoreValidator
	generator                studioBatchGenerator
	createGenerateTask       func(context.Context, *GenerateRequest) (*Task, error)
	getTask                  func(context.Context, string) (*Task, error)
	retryBackgroundRemoval   func(context.Context, string, string) (*studioBackgroundRemovalMaterialization, error)
	currentTime              func() time.Time
	serviceRunner            *listingStudioBatchServiceRunner
	batchRunner              *listingStudioBatchGenerationRunner
	detailRunner             *listingStudioBatchDetailRunner
	reviewRunner             *listingStudioBatchReviewRunner
	retryRunner              *listingStudioBatchRetryPrepareRunner
	taskCreationRunner       *listingStudioBatchTaskCreationRunner
	taskExecuteRunner        *listingStudioBatchTaskExecuteRunner
	taskPrepareRunner        *listingStudioBatchTaskPrepareRunner
	taskResumeRunner         *listingStudioBatchTaskResumeRunner
}

func newTaskStudioBatchService(config taskStudioBatchServiceConfig) *taskStudioBatchService {
	service := &taskStudioBatchService{
		repo:                     config.repo,
		batchRunRepo:             config.batchRunRepo,
		batchTaskLinkRepo:        config.batchTaskLinkRepo,
		studioSessionRepo:        config.studioSessionRepo,
		baselineChecker:          config.baselineChecker,
		sdsProductDetailProvider: config.sdsProductDetailProvider,
		storeValidator:           config.storeValidator,
		generator:                config.generator,
		createGenerateTask:       config.createGenerateTask,
		getTask:                  config.getTask,
		retryBackgroundRemoval:   config.retryBackgroundRemoval,
		currentTime:              time.Now,
		serviceRunner:            config.serviceRunner,
		batchRunner:              config.batchRunner,
		detailRunner:             config.detailRunner,
		reviewRunner:             config.reviewRunner,
		retryRunner:              config.retryRunner,
		taskCreationRunner:       config.taskCreationRunner,
		taskExecuteRunner:        config.taskExecuteRunner,
		taskPrepareRunner:        config.taskPrepareRunner,
		taskResumeRunner:         config.taskResumeRunner,
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
	if err := s.generator.RunPendingStudioBatchItems(ctx, batchID); err != nil {
		return nil, err
	}
	if err := s.generator.RecoverStudioBatchMaterialization(ctx, batchID); err != nil {
		return nil, err
	}
	return s.GetStudioBatchDetail(ctx, batchID)
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

func (s *taskStudioBatchService) RetryStudioBatchDesignBackgroundRemoval(ctx context.Context, batchID string, req *RetryStudioBatchDesignBackgroundRemovalRequest) (*StudioBatchDetail, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	if s.retryBackgroundRemoval == nil {
		return nil, fmt.Errorf("studio background removal retry service is not configured")
	}
	detail, err := s.GetStudioBatchDetail(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, fmt.Errorf("studio batch %s not found", strings.TrimSpace(batchID))
	}

	requested := normalizeStudioBatchDesignIDs(nil)
	if req != nil {
		requested = normalizeStudioBatchDesignIDs(req.DesignIDs)
	}
	requestedSet := make(map[string]struct{}, len(requested))
	for _, designID := range requested {
		requestedSet[designID] = struct{}{}
	}
	now := time.Now().UTC()
	if s.currentTime != nil {
		now = s.currentTime().UTC()
	}
	matched := 0
	for itemIndex := range detail.Items {
		for designIndex := range detail.Items[itemIndex].Designs {
			design := &detail.Items[itemIndex].Designs[designIndex]
			if len(requestedSet) > 0 {
				if _, ok := requestedSet[design.ID]; !ok {
					continue
				}
			} else if design.TransparentBackgroundMode != StudioTransparencyModeRemoval || design.BackgroundRemovalStatus == StudioBackgroundRemovalStatusSucceeded {
				continue
			}
			if design.TransparentBackgroundMode != StudioTransparencyModeRemoval {
				return nil, NewStudioBatchActionValidationError(fmt.Sprintf("design %s does not use background removal", design.ID))
			}
			matched++
			if strings.TrimSpace(design.OriginalImageURL) == "" {
				return nil, NewStudioBatchActionValidationError(fmt.Sprintf("design %s has no original image", design.ID))
			}

			design.BackgroundRemovalStatus = StudioBackgroundRemovalStatusPending
			design.BackgroundRemovalError = ""
			design.UpdatedAt = now
			if err := s.repo.UpdateStudioMaterializedDesign(ctx, design); err != nil {
				return nil, err
			}
			materialized, removeErr := s.retryBackgroundRemoval(ctx, design.OriginalImageURL, "studio-design-background-removal-retry.png")
			if removeErr != nil || materialized == nil || strings.TrimSpace(materialized.ImageURL) == "" {
				design.ImageURL = design.OriginalImageURL
				design.BackgroundRemovalStatus = StudioBackgroundRemovalStatusFailed
				removalMessage := "background removal returned no result"
				if removeErr != nil {
					removalMessage = removeErr.Error()
				}
				design.BackgroundRemovalError = compactStudioGenerationError(errors.New(removalMessage))
				design.UpdatedAt = now
				if err := s.repo.UpdateStudioMaterializedDesign(ctx, design); err != nil {
					return nil, err
				}
				continue
			}
			design.ImageURL = strings.TrimSpace(materialized.ImageURL)
			design.BackgroundRemovalStatus = StudioBackgroundRemovalStatusSucceeded
			design.BackgroundRemovalModel = strings.TrimSpace(materialized.Model)
			design.BackgroundRemovalError = ""
			design.UpdatedAt = now
			if err := s.repo.UpdateStudioMaterializedDesign(ctx, design); err != nil {
				return nil, err
			}
		}
	}
	if len(requestedSet) > 0 && matched != len(requestedSet) {
		return nil, NewStudioBatchActionValidationError("one or more requested designs are not eligible for background removal")
	}
	return s.GetStudioBatchDetail(ctx, batchID)
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

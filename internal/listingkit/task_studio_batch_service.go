package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	studiodomain "task-processor/internal/listing/studio"
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
	generateProductImages    func(context.Context, *StudioProductImageRequest) (*StudioProductImageResponse, error)
	getTask                  func(context.Context, string) (*Task, error)
	markTaskFailed           func(context.Context, string, string) error
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

	productImageUsage             StudioProductImageUsage
	generationUsageAdmission      GenerationUsageAdmission
	resolveUploadedImagePublicURL func(context.Context, string) (string, error)
}

func newTaskStudioBatchService(config taskStudioBatchServiceConfig) *taskStudioBatchService {
	service := &taskStudioBatchService{
		repo:                          config.repo,
		batchRunRepo:                  config.batchRunRepo,
		batchTaskLinkRepo:             config.batchTaskLinkRepo,
		studioSessionRepo:             config.studioSessionRepo,
		baselineChecker:               config.baselineChecker,
		sdsProductDetailProvider:      config.sdsProductDetailProvider,
		storeValidator:                config.storeValidator,
		generator:                     config.generator,
		createGenerateTask:            config.createGenerateTask,
		generateProductImages:         config.generateProductImages,
		productImageUsage:             config.productImageUsage,
		generationUsageAdmission:      config.generationUsageAdmission,
		resolveUploadedImagePublicURL: config.resolveUploadedImagePublicURL,
		getTask:                       config.getTask,
		markTaskFailed:                config.markTaskFailed,
		retryBackgroundRemoval:        config.retryBackgroundRemoval,
		currentTime:                   time.Now,
		serviceRunner:                 config.serviceRunner,
		batchRunner:                   config.batchRunner,
		detailRunner:                  config.detailRunner,
		reviewRunner:                  config.reviewRunner,
		retryRunner:                   config.retryRunner,
		taskCreationRunner:            config.taskCreationRunner,
		taskExecuteRunner:             config.taskExecuteRunner,
		taskPrepareRunner:             config.taskPrepareRunner,
		taskResumeRunner:              config.taskResumeRunner,
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
	designs := make([]studiodomain.BackgroundRemovalDesign, 0)
	for itemIndex := range detail.Items {
		for designIndex := range detail.Items[itemIndex].Designs {
			design := detail.Items[itemIndex].Designs[designIndex]
			designs = append(designs, studiodomain.BackgroundRemovalDesign{
				ID:                        design.ID,
				OriginalImageURL:          design.OriginalImageURL,
				ImageURL:                  design.ImageURL,
				TransparentBackgroundMode: studiodomain.TransparencyMode(design.TransparentBackgroundMode),
				BackgroundRemovalStatus:   studiodomain.BackgroundRemovalStatus(design.BackgroundRemovalStatus),
			})
		}
	}
	targets, err := studiodomain.SelectBackgroundRemovalTargets(designs, requested)
	if err != nil {
		return nil, adaptStudioBackgroundRemovalSelectionError(err)
	}
	designIDs := make([]string, 0, len(targets))
	for _, target := range targets {
		designIDs = append(designIDs, target.DesignID)
	}
	if err := s.rejectStudioBackgroundRemovalForOwnedTasks(ctx, batchID, designIDs); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if s.currentTime != nil {
		now = s.currentTime().UTC()
	}
	for index, target := range targets {
		flatIndex := 0
		var design *StudioMaterializedDesignRecord
		for itemIndex := range detail.Items {
			if target.DesignIndex < flatIndex+len(detail.Items[itemIndex].Designs) {
				designIndex := target.DesignIndex - flatIndex
				design = &detail.Items[itemIndex].Designs[designIndex]
				break
			}
			flatIndex += len(detail.Items[itemIndex].Designs)
		}
		if design == nil {
			return nil, fmt.Errorf("studio background removal target %d is out of range", index)
		}
		design.OriginalImageURL = target.SourceURL
		design.TransparentBackgroundMode = StudioTransparencyModeRemoval
		design.BackgroundRemovalStatus = StudioBackgroundRemovalStatusPending
		design.BackgroundRemovalError = ""
		design.BackgroundRemovalModel = ""
		design.UpdatedAt = now
		claimed, err := s.claimStudioBackgroundRemoval(ctx, design)
		if err != nil {
			return nil, err
		}
		if !claimed {
			return nil, NewStudioBatchActionValidationError(fmt.Sprintf("design %s background removal is already in progress", design.ID))
		}
		materialized, removeErr := s.retryBackgroundRemoval(ctx, target.SourceURL, "studio-design-background-removal-retry.png")
		if removeErr != nil || materialized == nil || strings.TrimSpace(materialized.ImageURL) == "" {
			design.ImageURL = design.OriginalImageURL
			design.BackgroundRemovalStatus = StudioBackgroundRemovalStatusFailed
			removalMessage := "background removal returned no result"
			if removeErr != nil {
				removalMessage = removeErr.Error()
			}
			design.BackgroundRemovalError = compactStudioGenerationError(errors.New(removalMessage))
			design.BackgroundRemovalModel = ""
			design.UpdatedAt = now
			if err := s.updateStudioBackgroundRemoval(ctx, design); err != nil {
				return nil, err
			}
			continue
		}
		design.ImageURL = strings.TrimSpace(materialized.ImageURL)
		design.BackgroundRemovalStatus = StudioBackgroundRemovalStatusSucceeded
		design.BackgroundRemovalModel = strings.TrimSpace(materialized.Model)
		design.BackgroundRemovalError = ""
		design.UpdatedAt = now
		if err := s.updateStudioBackgroundRemoval(ctx, design); err != nil {
			return nil, err
		}
	}
	return s.GetStudioBatchDetail(ctx, batchID)
}

func (s *taskStudioBatchService) ApplyManualStudioBatchDesignBackgroundRemoval(ctx context.Context, batchID string, designID string, imageURL string) (*StudioBatchDetail, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("studio batch repository is not configured")
	}
	_, target, err := s.validateManualStudioBatchDesignBackgroundRemoval(ctx, batchID, designID)
	if err != nil {
		return nil, err
	}
	return s.applyManualStudioBatchDesignBackgroundRemovalTarget(ctx, batchID, target, imageURL)
}

func (s *taskStudioBatchService) applyManualStudioBatchDesignBackgroundRemovalTarget(ctx context.Context, batchID string, target *StudioMaterializedDesignRecord, imageURL string) (*StudioBatchDetail, error) {
	if target == nil {
		return nil, NewStudioBatchActionValidationError("manual background removal design is required")
	}
	fields, err := studiodomain.PrepareManualBackgroundRemoval(studiodomain.ManualBackgroundRemovalInput{
		DesignID:            target.ID,
		OriginalImageURL:    target.OriginalImageURL,
		ImageURL:            target.ImageURL,
		ReplacementImageURL: imageURL,
	})
	if err != nil {
		return nil, adaptStudioBackgroundRemovalSelectionError(err)
	}
	if err := s.rejectStudioBackgroundRemovalForOwnedTasks(ctx, batchID, []string{target.ID}); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if s.currentTime != nil {
		now = s.currentTime().UTC()
	}
	target.OriginalImageURL = fields.OriginalImageURL
	target.ImageURL = fields.ImageURL
	target.TransparentBackgroundMode = StudioTransparencyMode(fields.TransparentBackgroundMode)
	target.BackgroundRemovalStatus = StudioBackgroundRemovalStatus(fields.BackgroundRemovalStatus)
	target.BackgroundRemovalError = fields.BackgroundRemovalError
	target.BackgroundRemovalModel = fields.BackgroundRemovalModel
	target.UpdatedAt = now
	repository, ok := s.repo.(manualBackgroundRemovalApplier)
	if !ok {
		return nil, fmt.Errorf("studio batch repository does not support atomic manual background removal")
	}
	applied, err := repository.ApplyManualStudioMaterializedDesignBackgroundRemoval(ctx, target)
	if err != nil {
		return nil, err
	}
	if !applied {
		return nil, NewStudioBatchActionValidationError(fmt.Sprintf("design %s background removal is already in progress", target.ID))
	}
	detail, err := s.GetStudioBatchDetail(ctx, batchID)
	if err != nil {
		return nil, &studioManualBackgroundRemovalCommittedError{err: err}
	}
	return detail, nil
}

func (s *taskStudioBatchService) validateManualStudioBatchDesignBackgroundRemoval(ctx context.Context, batchID string, designID string) (*StudioBatchDetail, *StudioMaterializedDesignRecord, error) {
	if s == nil || s.repo == nil {
		return nil, nil, fmt.Errorf("studio batch repository is not configured")
	}
	detail, err := s.GetStudioBatchDetail(ctx, batchID)
	if err != nil {
		return nil, nil, err
	}
	if detail == nil {
		return nil, nil, fmt.Errorf("studio batch %s not found", strings.TrimSpace(batchID))
	}

	designs := make([]studiodomain.BackgroundRemovalDesign, 0)
	designPointers := make([]*StudioMaterializedDesignRecord, 0)
	for itemIndex := range detail.Items {
		for designIndex := range detail.Items[itemIndex].Designs {
			design := &detail.Items[itemIndex].Designs[designIndex]
			designs = append(designs, studiodomain.BackgroundRemovalDesign{
				ID:                        design.ID,
				OriginalImageURL:          design.OriginalImageURL,
				ImageURL:                  design.ImageURL,
				TransparentBackgroundMode: studiodomain.TransparencyMode(design.TransparentBackgroundMode),
				BackgroundRemovalStatus:   studiodomain.BackgroundRemovalStatus(design.BackgroundRemovalStatus),
			})
			designPointers = append(designPointers, design)
		}
	}
	designIndex, found, err := studiodomain.FindManualBackgroundRemovalDesign(designs, designID)
	if err != nil {
		return nil, nil, adaptStudioBackgroundRemovalSelectionError(err)
	}
	if !found {
		return nil, nil, NewStudioBatchActionValidationError(fmt.Sprintf("design %s is not part of batch %s", strings.TrimSpace(designID), strings.TrimSpace(batchID)))
	}
	target := designPointers[designIndex]
	if err := s.rejectStudioBackgroundRemovalForOwnedTasks(ctx, batchID, []string{target.ID}); err != nil {
		return nil, nil, err
	}
	return detail, target, nil
}

type studioManualBackgroundRemovalCommittedError struct {
	err error
}

func (e *studioManualBackgroundRemovalCommittedError) Error() string {
	if e == nil || e.err == nil {
		return "manual background removal committed but detail read failed"
	}
	return e.err.Error()
}

func (e *studioManualBackgroundRemovalCommittedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (s *taskStudioBatchService) claimStudioBackgroundRemoval(ctx context.Context, design *StudioMaterializedDesignRecord) (bool, error) {
	if repository, ok := s.repo.(studioBackgroundRemovalRepository); ok {
		return repository.ClaimStudioMaterializedDesignBackgroundRemoval(ctx, design)
	}
	return true, s.repo.UpdateStudioMaterializedDesign(ctx, design)
}

func (s *taskStudioBatchService) rejectStudioBackgroundRemovalForOwnedTasks(ctx context.Context, batchID string, designIDs []string) error {
	if s == nil || s.batchTaskLinkRepo == nil || len(designIDs) == 0 {
		return nil
	}
	links, err := s.batchTaskLinkRepo.ListStudioBatchTaskLinksByBatchID(ctx, batchID)
	if err != nil {
		return err
	}
	targetDesignIDs := make(map[string]struct{}, len(designIDs))
	for _, designID := range designIDs {
		targetDesignIDs[strings.TrimSpace(designID)] = struct{}{}
	}
	for _, link := range links {
		if strings.TrimSpace(link.ListingKitTaskID) == "" &&
			(link.Status != studioBatchTaskLinkStatusCreating || s.studioBatchTaskLinkIsStale(&link)) {
			continue
		}
		designID := strings.TrimSpace(link.DesignID)
		if _, ok := targetDesignIDs[designID]; !ok {
			continue
		}
		return NewStudioBatchActionValidationError(fmt.Sprintf("design %s already owns ListingKit task %s", designID, strings.TrimSpace(link.ListingKitTaskID)))
	}
	return nil
}

func (s *taskStudioBatchService) updateStudioBackgroundRemoval(ctx context.Context, design *StudioMaterializedDesignRecord) error {
	if repository, ok := s.repo.(studioBackgroundRemovalRepository); ok {
		return repository.UpdateStudioMaterializedDesignBackgroundRemoval(ctx, design)
	}
	return s.repo.UpdateStudioMaterializedDesign(ctx, design)
}

func adaptStudioBackgroundRemovalSelectionError(err error) error {
	var validationErr *studiodomain.BackgroundRemovalValidationError
	if errors.As(err, &validationErr) {
		return NewStudioBatchActionValidationError(validationErr.Error())
	}
	return err
}

func (s *taskStudioBatchService) CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {
	s.ensureServiceRunner()
	if s.serviceRunner == nil {
		return nil, fmt.Errorf("studio batch service is not configured")
	}
	if req != nil && req.AllowPartialWhileGenerating {
		ctx = withStudioBatchPartialTaskCreationAllowed(ctx)
	}
	ctx = withStudioBatchTaskImageStrategy(ctx, req)
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
	ctx = withStudioBatchTaskImageStrategy(ctx, req)
	return s.serviceRunner.PrepareCreateTasks(ctx, batchID, req)
}

package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	asset "task-processor/internal/asset"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
)

type taskGenerationServiceConfig struct {
	repo                              Repository
	assetRepo                         assetrepo.Repository
	assetRecipeResolver               assetrecipe.Resolver
	assetBundleBuilder                assetbundle.Builder
	assetGenerator                    assetgeneration.Service
	listAssetGenerationTasks          func(context.Context, string) ([]assetgeneration.Task, error)
	listGenerationReviews             func(context.Context, string) ([]GenerationReviewRecord, error)
	buildRetryGenerationTaskSelection func(context.Context, *Task, *asset.Inventory, []assetgeneration.Task, *RetryGenerationTasksRequest) ([]assetgeneration.Task, error)
	persistGenerationReviewDecision   func(context.Context, string, string, *GenerationReviewSession, *AssetGenerationActionTarget) (*GenerationReviewRecord, error)
	standardWorkflow                  func() (StandardProductWorkflowClient, bool)
	platformAdaptWorkflow             func() (PlatformAdaptWorkflowClient, bool)
}

type taskGenerationService struct {
	repo                              Repository
	assetRepo                         assetrepo.Repository
	assetRecipeResolver               assetrecipe.Resolver
	assetBundleBuilder                assetbundle.Builder
	assetGenerator                    assetgeneration.Service
	listAssetGenerationTasks          func(context.Context, string) ([]assetgeneration.Task, error)
	listGenerationReviews             func(context.Context, string) ([]GenerationReviewRecord, error)
	buildRetryGenerationTaskSelection func(context.Context, *Task, *asset.Inventory, []assetgeneration.Task, *RetryGenerationTasksRequest) ([]assetgeneration.Task, error)
	persistGenerationReviewDecision   func(context.Context, string, string, *GenerationReviewSession, *AssetGenerationActionTarget) (*GenerationReviewRecord, error)
	standardWorkflow                  func() (StandardProductWorkflowClient, bool)
	platformAdaptWorkflow             func() (PlatformAdaptWorkflowClient, bool)
}

func newTaskGenerationService(config taskGenerationServiceConfig) *taskGenerationService {
	return &taskGenerationService{
		repo:                              config.repo,
		assetRepo:                         config.assetRepo,
		assetRecipeResolver:               config.assetRecipeResolver,
		assetBundleBuilder:                config.assetBundleBuilder,
		assetGenerator:                    config.assetGenerator,
		listAssetGenerationTasks:          config.listAssetGenerationTasks,
		listGenerationReviews:             config.listGenerationReviews,
		buildRetryGenerationTaskSelection: config.buildRetryGenerationTaskSelection,
		persistGenerationReviewDecision:   config.persistGenerationReviewDecision,
		standardWorkflow:                  config.standardWorkflow,
		platformAdaptWorkflow:             config.platformAdaptWorkflow,
	}
}

func (s *taskGenerationService) GetTaskGenerationTasks(ctx context.Context, taskID string, query *GenerationTaskQuery) (*GenerationTaskPage, error) {
	snapshot, err := buildTaskGenerationTasksReadSnapshotPhase(s).run(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildTaskGenerationTasksReadPagePhase().run(snapshot, query), nil
}

func (s *taskGenerationService) GetTaskGenerationQueue(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationQueuePage, error) {
	snapshot, err := buildTaskGenerationQueueReadSnapshotPhase(s).run(ctx, taskID)
	if err != nil {
		return nil, err
	}
	page := buildTaskGenerationQueueReadPagePhase().run(snapshot, query)
	return buildTaskGenerationQueueReadResponsePhase().run(taskID, page, query), nil
}

func (s *taskGenerationService) GetTaskGenerationReviewSession(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewSessionResponse, error) {
	snapshot, err := buildTaskGenerationReviewReadSnapshotPhase(s).run(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildTaskGenerationReviewSessionReadPhase().run(taskID, snapshot, query), nil
}

func (s *taskGenerationService) GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewPreviewResponse, error) {
	snapshot, err := buildTaskGenerationReviewReadSnapshotPhase(s).run(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildTaskGenerationReviewPreviewReadPhase().run(taskID, snapshot, query), nil
}

func (s *taskGenerationService) RetryTaskGenerationTasks(ctx context.Context, taskID string, req *RetryGenerationTasksRequest) (*GenerationTaskPage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil {
		return nil, fmt.Errorf("listing kit result is not available")
	}
	inventory, err := s.assetRepo.GetInventory(ctx, asset.InventoryRef{TaskID: task.ID})
	if err != nil {
		return nil, err
	}
	existingTasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	selectedTasks, err := s.buildRetryGenerationTaskSelection(ctx, task, inventory, existingTasks, req)
	if err != nil {
		return nil, err
	}
	if len(selectedTasks) == 0 {
		page := buildGenerationTaskPage(task.ID, task.UpdatedAt, nil, nil, generationTaskListPage{
			Page:     defaultGenerationTaskPage,
			PageSize: defaultGenerationTaskPageSize,
			Total:    0,
		})
		page.MatchedQueue = &GenerationWorkQueue{Summary: &GenerationWorkQueueSummary{}}
		page.ExecutedQueue = &GenerationWorkQueue{Summary: &GenerationWorkQueueSummary{}}
		return page, nil
	}

	dispatchResult, err := s.assetGenerator.Dispatch(ctx, assetgeneration.DispatchRequest{
		TaskID:    task.ID,
		Product:   effectiveCatalogProduct(task.Result),
		Inventory: inventory,
		Tasks:     selectedTasks,
	})
	if err != nil {
		return nil, err
	}
	mutationResult := dispatchResult
	if dispatchResult == nil {
		dispatchResult = &assetgeneration.Result{}
	}

	updatedTasks := buildRetryGenerationMutationPhase().run(inventory, existingTasks, selectedTasks, mutationResult)
	reviews, err := s.listGenerationReviews(ctx, task.ID)
	if err != nil {
		return nil, err
	}

	if mutationResult != nil {
		if err := buildRetryGenerationPersistPhase(s.assetRepo).run(ctx, task.ID, inventory, updatedTasks); err != nil {
			return nil, err
		}
	}

	rebuiltResult, page := buildRetryGenerationProjectionPhase(s.assetRecipeResolver, s.assetBundleBuilder).run(
		task,
		inventory,
		updatedTasks,
		selectedTasks,
		dispatchResult,
		reviews,
	)
	if err := s.repo.SaveTaskResult(ctx, task.ID, rebuiltResult); err != nil {
		return nil, err
	}
	return page, nil
}

func (s *taskGenerationService) ExecuteTaskGenerationAction(ctx context.Context, taskID string, req *ExecuteGenerationActionRequest) (*GenerationActionExecutionResult, error) {
	if handled, result, err := s.executeLayerTemporalAction(ctx, taskID, req); handled {
		return result, err
	}
	entry, err := buildTaskGenerationActionEntryPhase(s).run(ctx, taskID, req)
	if err != nil {
		return nil, err
	}
	result := entry.result
	// buildGenerationReviewSession(...) now lives in taskGenerationActionEntryPhase.
	execution, err := buildTaskGenerationActionExecutePhase(s).run(ctx, taskID, entry.baseResult, entry.target)
	if err != nil {
		return nil, err
	}
	result.Retry = execution.retryPage
	result.Queue = execution.queuePage
	if err := buildTaskGenerationActionPersistPhase(s).run(ctx, taskID, entry.target, execution); err != nil {
		return nil, err
	}
	refresh, err := buildTaskGenerationActionRefreshPhase(s).run(ctx, taskID, entry.baseResult, entry.target.QueueQuery)
	if err != nil {
		return nil, err
	}
	projection := buildTaskGenerationActionProjectionPhase().run(&taskGenerationActionProjectionInput{
		actionKey:             entry.target.ActionKey,
		target:                entry.target,
		responseMode:          result.ResponseMode,
		previousReviewSession: entry.previousReviewSession,
		currentResult:         entry.baseResult,
		refresh:               refresh,
		execution:             execution,
	})
	result = buildTaskGenerationActionFinalizePhase().run(result, projection)
	return result, nil
}

func (s *taskGenerationService) DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *GenerationReviewNavigationDispatchRequest) (*GenerationReviewNavigationDispatchResponse, error) {
	dispatchInput, err := buildTaskGenerationNavigationDispatchEntry().run(req)
	if err != nil {
		return nil, err
	}
	response, err := s.dispatchGenerationNavigationPrimary(ctx, taskID, dispatchInput.target, dispatchInput.responseMode)
	if err != nil {
		return nil, err
	}
	projection := buildTaskGenerationNavigationDispatchProjectionPhase()
	var executedPlan *GenerationNavigationDispatchExecution
	if dispatchInput.planMode == "execute_plan" {
		executedPlan, err = s.executeGenerationNavigationDispatchPlan(ctx, taskID, dispatchInput.target, dispatchInput.responseMode)
		if err != nil {
			return nil, err
		}
	}
	return projection.run(response, dispatchInput.planMode, executedPlan), nil
}

func (s *taskGenerationService) executeLayerTemporalAction(ctx context.Context, taskID string, req *ExecuteGenerationActionRequest) (bool, *GenerationActionExecutionResult, error) {
	actionKey := requestedAssetGenerationActionKey(req)
	switch actionKey {
	case assetGenerationActionRunStandardProductTemporal:
		client, enabled := s.standardWorkflow()
		if !enabled || client == nil {
			return true, nil, fmt.Errorf("standard product temporal workflow is not configured")
		}
		if err := client.StartStandardProduct(ctx, StandardProductWorkflowStartInput{
			TaskID:      strings.TrimSpace(taskID),
			RequestedAt: time.Now().UTC(),
		}); err != nil {
			return true, nil, err
		}
		return true, buildTaskGenerationActionTemporalResultPhase().run(actionKey, req.ResponseMode, nil), nil
	case assetGenerationActionRunPlatformAdaptTemporal:
		client, enabled := s.platformAdaptWorkflow()
		if !enabled || client == nil {
			return true, nil, fmt.Errorf("platform adaptation temporal workflow is not configured")
		}
		platform := resolveLayerTemporalPlatform(req)
		if err := client.StartPlatformAdaptation(ctx, PlatformAdaptWorkflowStartInput{
			TaskID:      strings.TrimSpace(taskID),
			Platform:    platform,
			RequestedAt: time.Now().UTC(),
		}); err != nil {
			return true, nil, err
		}
		return true, buildTaskGenerationActionTemporalResultPhase().run(
			actionKey,
			req.ResponseMode,
			&GenerationQueueQuery{Platform: platform},
		), nil
	default:
		return false, nil, nil
	}
}

func (s *taskGenerationService) getCurrentAssetGenerationOverview(ctx context.Context, taskID string) (*AssetGenerationOverview, error) {
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildTaskGenerationCurrentStateViewsPhase().overview(result), nil
}

func (s *taskGenerationService) getCurrentAssetGenerationQueue(ctx context.Context, taskID string) (*GenerationWorkQueue, error) {
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildTaskGenerationCurrentStateViewsPhase().queue(result), nil
}

func (s *taskGenerationService) getCurrentActionRenderPreviews(ctx context.Context, taskID string, query *GenerationQueueQuery) ([]PlatformAssetRenderPreviews, error) {
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildTaskGenerationCurrentStateViewsPhase().renderPreviews(result, query), nil
}

func (s *taskGenerationService) getCurrentListingKitResult(ctx context.Context, taskID string) (*ListingKitResult, error) {
	snapshot, err := buildTaskGenerationCurrentStateSnapshotPhase(s).run(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return snapshot.result, nil
}

func (s *taskGenerationService) dispatchGenerationNavigationPrimary(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationReviewNavigationDispatchResponse, error) {
	return buildTaskGenerationNavigationDispatchPrimaryPhase(s).run(ctx, taskID, target, responseMode)
}

func (s *taskGenerationService) executeGenerationNavigationDispatchPlan(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationNavigationDispatchExecution, error) {
	return buildTaskGenerationNavigationDispatchPlanPhase(s).run(ctx, taskID, target, responseMode)
}

func (s *taskGenerationService) executeGenerationNavigationDispatchPlanSequential(ctx context.Context, taskID string, responseMode string, plan *GenerationNavigationDispatchPlan, execution *GenerationNavigationDispatchExecution) {
	buildTaskGenerationNavigationDispatchStepExecutionPhase(s).runSequential(ctx, taskID, responseMode, plan, execution)
}

func (s *taskGenerationService) executeGenerationNavigationDispatchPlanParallel(ctx context.Context, taskID string, responseMode string, plan *GenerationNavigationDispatchPlan, execution *GenerationNavigationDispatchExecution) {
	buildTaskGenerationNavigationDispatchPlanParallelPhase(s).run(ctx, taskID, responseMode, plan, execution)
}

func (s *taskGenerationService) executeGenerationNavigationDispatchPlanStep(ctx context.Context, taskID string, step GenerationNavigationDispatchStep, responseMode string) *GenerationNavigationDispatchExecutionStep {
	return buildTaskGenerationNavigationDispatchStepExecutionPhase(s).run(ctx, taskID, step, responseMode)
}

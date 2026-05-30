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
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	filtered := filterGenerationTasks(tasks, query)
	sorted := sortGenerationTasks(filtered, query)
	paged, meta := paginateGenerationTasks(sorted, query)
	return buildGenerationTaskPage(task.ID, task.UpdatedAt, filtered, paged, meta), nil
}

func (s *taskGenerationService) GetTaskGenerationQueue(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationQueuePage, error) {
	snapshot, err := buildTaskGenerationQueueReadSnapshotPhase(s).run(ctx, taskID)
	if err != nil {
		return nil, err
	}
	task := snapshot.task
	reviewedResult := snapshot.result
	queue := snapshot.queue
	if queue == nil {
		page := buildGenerationQueuePage(task.ID, task.UpdatedAt, nil, nil, generationQueueListPage{
			Page:     resolveGenerationQueuePage(query),
			PageSize: resolveGenerationQueuePageSize(query),
			Total:    0,
		})
		page.DeltaToken = buildGenerationQueueDeltaToken(page, query)
		if isGenerationReviewReadNotModified(query, page.DeltaToken) {
			return applyGenerationConditionalStateToQueuePage(&GenerationQueuePage{
				TaskID:      task.ID,
				DeltaToken:  page.DeltaToken,
				NotModified: true,
				Page:        page.Page,
				PageSize:    page.PageSize,
				Total:       page.Total,
				UpdatedAt:   page.UpdatedAt,
			}), nil
		}
		return applyGenerationConditionalStateToQueuePage(page), nil
	}
	filtered := filterGenerationQueueItems(queue.Items, query)
	sorted := sortGenerationQueueItems(filtered, query)
	paged, meta := paginateGenerationQueueItems(sorted, query)
	page := buildGenerationQueuePage(task.ID, task.UpdatedAt, filtered, paged, meta)
	attachReviewSummaryToGenerationQueuePage(page, reviewedResult)
	page.DeltaToken = buildGenerationQueueDeltaToken(page, query)
	if isGenerationReviewReadNotModified(query, page.DeltaToken) {
		return applyGenerationConditionalStateToQueuePage(&GenerationQueuePage{
			TaskID:      task.ID,
			DeltaToken:  page.DeltaToken,
			NotModified: true,
			Page:        page.Page,
			PageSize:    page.PageSize,
			Total:       page.Total,
			UpdatedAt:   page.UpdatedAt,
		}), nil
	}
	return applyGenerationConditionalStateToQueuePage(page), nil
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
	queue, err := s.getCurrentAssetGenerationQueue(ctx, taskID)
	if err != nil {
		return nil, err
	}
	baseResult, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	overview := buildAssetGenerationOverview(queue)
	target, source, err := resolveAssetGenerationActionTarget(overview, req)
	if err != nil {
		return nil, err
	}
	if target.ExpectedImpact == nil {
		target.ExpectedImpact = buildAssetGenerationActionImpact(queue, target.QueueQuery)
	}
	previousReviewSession := buildGenerationReviewSession(baseResult, queue, target.QueueQuery)
	result := &GenerationActionExecutionResult{
		ActionKey:       target.ActionKey,
		InteractionMode: target.InteractionMode,
		ResponseMode:    normalizeGenerationActionResponseMode(req.ResponseMode),
		ResolvedTarget:  target,
		Audit: &GenerationActionAudit{
			RequestedActionKey: requestedAssetGenerationActionKey(req),
			ResolvedActionKey:  target.ActionKey,
			ResolutionSource:   source,
			ExecutionPath:      target.InteractionMode,
			ExecutedAt:         time.Now().UTC(),
		},
	}
	execution, err := buildTaskGenerationActionExecutePhase(s).run(ctx, taskID, baseResult, target)
	if err != nil {
		return nil, err
	}
	result.Retry = execution.retryPage
	result.Queue = execution.queuePage
	if isPersistedGenerationReviewAction(target.ActionKey) && s.persistGenerationReviewDecision != nil {
		if _, err := s.persistGenerationReviewDecision(ctx, taskID, target.ActionKey, execution.persistenceSession, target); err != nil {
			return nil, err
		}
	}
	refresh, err := buildTaskGenerationActionRefreshPhase(s).run(ctx, taskID, baseResult, target.QueueQuery)
	if err != nil {
		return nil, err
	}
	projection := buildTaskGenerationActionProjectionPhase().run(&taskGenerationActionProjectionInput{
		actionKey:             target.ActionKey,
		target:                target,
		responseMode:          result.ResponseMode,
		previousReviewSession: previousReviewSession,
		currentResult:         baseResult,
		refresh:               refresh,
		execution:             execution,
	})
	result.Overview = projection.Overview
	result.Queue = projection.Queue
	result.Retry = projection.Retry
	result.ReviewWorkflow = projection.ReviewWorkflow
	result.ReviewSession = projection.ReviewSession
	result.ReviewPatch = projection.ReviewPatch
	result.PlatformRenderPreviews = projection.PlatformRenderPreviews
	result.DeltaToken = projection.DeltaToken
	return applyGenerationConditionalStateToActionResult(result), nil
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
		return true, &GenerationActionExecutionResult{
			ActionKey:       actionKey,
			InteractionMode: "queue_only",
			ResponseMode:    normalizeGenerationActionResponseMode(req.ResponseMode),
			ResolvedTarget: &AssetGenerationActionTarget{
				ActionKey:       actionKey,
				InteractionMode: "queue_only",
			},
			Audit: &GenerationActionAudit{
				RequestedActionKey: actionKey,
				ResolvedActionKey:  actionKey,
				ResolutionSource:   "layer_temporal",
				ExecutionPath:      "queue_only",
				ExecutedAt:         time.Now().UTC(),
			},
		}, nil
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
		return true, &GenerationActionExecutionResult{
			ActionKey:       actionKey,
			InteractionMode: "queue_only",
			ResponseMode:    normalizeGenerationActionResponseMode(req.ResponseMode),
			ResolvedTarget: &AssetGenerationActionTarget{
				ActionKey:       actionKey,
				InteractionMode: "queue_only",
				QueueQuery:      &GenerationQueueQuery{Platform: platform},
			},
			Audit: &GenerationActionAudit{
				RequestedActionKey: actionKey,
				ResolvedActionKey:  actionKey,
				ResolutionSource:   "layer_temporal",
				ExecutionPath:      "queue_only",
				ExecutedAt:         time.Now().UTC(),
			},
		}, nil
	default:
		return false, nil, nil
	}
}

func (s *taskGenerationService) getCurrentAssetGenerationOverview(ctx context.Context, taskID string) (*AssetGenerationOverview, error) {
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return result.AssetGenerationOverview, nil
}

func (s *taskGenerationService) getCurrentAssetGenerationQueue(ctx context.Context, taskID string) (*GenerationWorkQueue, error) {
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return result.AssetGenerationQueue, nil
}

func (s *taskGenerationService) getCurrentActionRenderPreviews(ctx context.Context, taskID string, query *GenerationQueueQuery) ([]PlatformAssetRenderPreviews, error) {
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildActionPlatformRenderPreviews(result, query), nil
}

func (s *taskGenerationService) getCurrentListingKitResult(ctx context.Context, taskID string) (*ListingKitResult, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	reviews, err := s.listGenerationReviews(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	return withListingKitResultGenerationAndReview(task.Result, tasks, reviews), nil
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

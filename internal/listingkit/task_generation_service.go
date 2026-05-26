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
	reviewedResult := withListingKitResultGenerationAndReview(task.Result, tasks, reviews)
	queue := reviewedResult.AssetGenerationQueue
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
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	queue, err := s.getCurrentAssetGenerationQueue(ctx, taskID)
	if err != nil {
		return nil, err
	}
	session := buildGenerationReviewSession(result, queue, query)
	if session == nil {
		return applyGenerationConditionalStateToReviewSessionResponse(&GenerationReviewSessionResponse{TaskID: taskID}), nil
	}
	deltaToken := buildGenerationReviewReadDeltaToken(session)
	responseMode := normalizeGenerationActionResponseMode("")
	if query != nil {
		responseMode = normalizeGenerationActionResponseMode(query.ResponseMode)
	}
	if isGenerationReviewReadNotModified(query, deltaToken) {
		return applyGenerationConditionalStateToReviewSessionResponse(&GenerationReviewSessionResponse{
			TaskID:       taskID,
			DeltaToken:   deltaToken,
			NotModified:  true,
			ResponseMode: responseMode,
		}), nil
	}
	response := &GenerationReviewSessionResponse{
		TaskID:       taskID,
		DeltaToken:   deltaToken,
		ResponseMode: responseMode,
	}
	if responseMode == "patch_only" {
		baseSession := buildGenerationReviewSession(result, queue, buildGenerationReviewSessionBaseQuery(query))
		response.Patch = buildGenerationReviewSessionPatch(baseSession, session)
		if response.Patch != nil && response.Patch.DeltaToken == "" {
			response.Patch.DeltaToken = deltaToken
		}
		return applyGenerationConditionalStateToReviewSessionResponse(response), nil
	}
	response.Session = session
	return applyGenerationConditionalStateToReviewSessionResponse(response), nil
}

func (s *taskGenerationService) GetTaskGenerationReviewPreview(ctx context.Context, taskID string, query *GenerationQueueQuery) (*GenerationReviewPreviewResponse, error) {
	result, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	queue, err := s.getCurrentAssetGenerationQueue(ctx, taskID)
	if err != nil {
		return nil, err
	}
	session := buildGenerationReviewSession(result, queue, query)
	if session == nil {
		return applyGenerationConditionalStateToReviewPreviewResponse(&GenerationReviewPreviewResponse{TaskID: taskID}), nil
	}
	deltaToken := buildGenerationReviewReadDeltaToken(session)
	if isGenerationReviewReadNotModified(query, deltaToken) {
		return applyGenerationConditionalStateToReviewPreviewResponse(&GenerationReviewPreviewResponse{
			TaskID:      taskID,
			DeltaToken:  deltaToken,
			NotModified: true,
		}), nil
	}
	viewer, preview, target, toolbar := resolveGenerationReviewPreviewResponse(session, query)
	revisionStatus, revisionReason := resolveGenerationReviewPreviewRevisionStatus(viewer, query)
	return applyGenerationConditionalStateToReviewPreviewResponse(&GenerationReviewPreviewResponse{
		TaskID:                 taskID,
		DeltaToken:             deltaToken,
		Viewer:                 viewer,
		Preview:                preview,
		ScenePreset:            buildGenerationScenePresetSummary(result.AssetBundle, focusedPreviewAssetID(preview)),
		ReviewTarget:           target,
		Toolbar:                toolbar,
		RevisionStatus:         revisionStatus,
		RevisionMismatchReason: revisionReason,
	}), nil
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

	updatedTasks := mergeGenerationTasks(existingTasks, dispatchResult.Tasks)
	reviews, err := s.listGenerationReviews(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	retriedTargets := generationTaskTargets(selectedTasks)
	inventory.Records = replaceGeneratedAssetsForTargets(inventory.Records, retriedTargets, dispatchResult.Assets)
	inventory.Summary = rebuildInventorySummary(inventory)

	if err := s.assetRepo.SaveInventory(ctx, inventory); err != nil {
		return nil, err
	}
	if err := s.assetRepo.SaveGenerationTasks(ctx, task.ID, updatedTasks); err != nil {
		return nil, err
	}

	rebuiltResult := *task.Result
	rebuiltResult.AssetBundle = rebuildBundleFromInventory(task.Result.AssetBundle, inventory)
	rebuiltResult.AssetInventorySummary = inventory.Summary
	recipesByPlatform := resolveRecipesForPlatforms(s.assetRecipeResolver, task.Request.Platforms, task.Result.CanonicalProduct)
	attachPlatformImageBundles(&rebuiltResult, inventory, recipesByPlatform, &assetgeneration.Result{Tasks: updatedTasks}, s.assetBundleBuilder)
	decorateListingKitResultGeneration(&rebuiltResult, updatedTasks)
	syncAssetRenderPreviews(&rebuiltResult)
	if err := s.repo.SaveTaskResult(ctx, task.ID, &rebuiltResult); err != nil {
		return nil, err
	}
	decorateListingKitResultReview(&rebuiltResult, reviews)

	page := buildGenerationTaskPage(task.ID, task.UpdatedAt, updatedTasks, updatedTasks, generationTaskListPage{
		Page:     defaultGenerationTaskPage,
		PageSize: defaultGenerationTaskPageSize,
		Total:    len(updatedTasks),
	})
	page.MatchedQueue = buildMatchedGenerationQueue(rebuiltResult.AssetGenerationQueue, selectedTasks)
	page.ExecutedQueue = buildMatchedGenerationQueue(rebuiltResult.AssetGenerationQueue, dispatchResult.Tasks)
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
	switch target.InteractionMode {
	case "retryable":
		retryPage, err := s.RetryTaskGenerationTasks(ctx, taskID, cloneRetryGenerationTasksRequest(target.RetryRequest))
		if err != nil {
			return nil, err
		}
		result.Retry = retryPage
	default:
		queuePage, err := s.GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(target.QueueQuery))
		if err != nil {
			return nil, err
		}
		result.Queue = queuePage
	}
	if isPersistedGenerationReviewAction(target.ActionKey) && s.persistGenerationReviewDecision != nil {
		var persistenceSession *GenerationReviewSession
		switch target.InteractionMode {
		case "retryable":
			persistenceSession = buildGenerationReviewSession(baseResult, generationWorkQueueFromRetryPage(result.Retry), target.QueueQuery)
		default:
			persistenceSession = buildGenerationReviewSession(baseResult, generationWorkQueueFromPage(result.Queue), target.QueueQuery)
		}
		if _, err := s.persistGenerationReviewDecision(ctx, taskID, target.ActionKey, persistenceSession, target); err != nil {
			return nil, err
		}
	}
	result.Overview, err = s.getCurrentAssetGenerationOverview(ctx, taskID)
	if err != nil {
		return nil, err
	}
	result.PlatformRenderPreviews, err = s.getCurrentActionRenderPreviews(ctx, taskID, target.QueueQuery)
	if err != nil {
		return nil, err
	}
	if len(result.PlatformRenderPreviews) == 0 {
		result.PlatformRenderPreviews = buildActionPlatformRenderPreviews(baseResult, target.QueueQuery)
	}
	currentResult, err := s.getCurrentListingKitResult(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if len(currentResult.PlatformAssetRenderPreviews) == 0 && len(result.PlatformRenderPreviews) > 0 {
		currentResult.PlatformAssetRenderPreviews = append([]PlatformAssetRenderPreviews(nil), result.PlatformRenderPreviews...)
	}
	if len(currentResult.AssetRenderPreviews) == 0 && baseResult != nil {
		currentResult.AssetRenderPreviews = append([]AssetRenderPreview(nil), baseResult.AssetRenderPreviews...)
	}
	switch target.InteractionMode {
	case "retryable":
		result.ReviewSession = buildGenerationReviewSession(currentResult, generationWorkQueueFromRetryPage(result.Retry), target.QueueQuery)
	default:
		result.ReviewSession = buildGenerationReviewSession(currentResult, generationWorkQueueFromPage(result.Queue), target.QueueQuery)
	}
	result.ReviewWorkflow = buildGenerationReviewWorkflowResult(target.ActionKey, target)
	applyGenerationReviewWorkflow(result.ReviewSession, result.ReviewWorkflow)
	result.ReviewPatch = buildGenerationReviewSessionPatch(previousReviewSession, result.ReviewSession)
	if result.ReviewPatch != nil {
		result.ReviewPatch.LastWorkflowResult = result.ReviewWorkflow
		result.DeltaToken = result.ReviewPatch.DeltaToken
	}
	if result.DeltaToken == "" {
		result.DeltaToken = buildGenerationReviewDeltaToken(result.ReviewSession)
	}
	if result.ResponseMode == "patch_only" {
		result.ReviewSession = nil
		result.PlatformRenderPreviews = nil
	}
	return applyGenerationConditionalStateToActionResult(result), nil
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

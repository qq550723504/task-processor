package listingkit

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	result.Overview = refresh.overview
	result.PlatformRenderPreviews = refresh.platformRenderPreviews
	currentResult := refresh.currentResult
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

func (s *taskGenerationService) DispatchTaskGenerationNavigation(ctx context.Context, taskID string, req *GenerationReviewNavigationDispatchRequest) (*GenerationReviewNavigationDispatchResponse, error) {
	if req == nil || req.Target == nil {
		return nil, fmt.Errorf("%w: missing navigation target", ErrGenerationActionNotFound)
	}
	target := cloneGenerationReviewNavigationTarget(req.Target)
	ApplyGenerationConditionalBaselineToNavigationTarget(target, "")

	responseMode := normalizeGenerationActionResponseMode(req.ResponseMode)
	planMode := normalizeGenerationNavigationDispatchPlanMode(req.PlanMode)
	response, err := s.dispatchGenerationNavigationPrimary(ctx, taskID, target, responseMode)
	if err != nil {
		return nil, err
	}
	response.PlanMode = planMode
	if planMode == "execute_plan" {
		executedPlan, err := s.executeGenerationNavigationDispatchPlan(ctx, taskID, target, responseMode)
		if err != nil {
			return nil, err
		}
		applyExecutedPlanToDispatchResponse(response, executedPlan)
	}
	return finalizeGenerationReviewNavigationDispatchResponse(response), nil
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
	switch normalizeGenerationReviewDispatchKind(target) {
	case "action":
		actionReq := &ExecuteGenerationActionRequest{
			ResponseMode: responseMode,
			Target:       cloneAssetGenerationActionTarget(target.ActionTarget),
		}
		if actionReq.Target == nil {
			return nil, fmt.Errorf("%w: missing action target", ErrGenerationActionNotFound)
		}
		actionReq.ActionKey = actionReq.Target.ActionKey
		action, err := s.ExecuteTaskGenerationAction(ctx, taskID, actionReq)
		if err != nil {
			return nil, err
		}
		return &GenerationReviewNavigationDispatchResponse{
			TaskID:       taskID,
			DispatchKind: "action",
			ResponseMode: responseMode,
			DeltaToken:   action.DeltaToken,
			Action:       action,
		}, nil
	case "preview":
		preview, err := s.GetTaskGenerationReviewPreview(ctx, taskID, cloneGenerationQueueQuery(target.PreviewQuery))
		if err != nil {
			return nil, err
		}
		return &GenerationReviewNavigationDispatchResponse{
			TaskID:        taskID,
			DispatchKind:  "preview",
			ResponseMode:  responseMode,
			DeltaToken:    preview.DeltaToken,
			ReviewPreview: preview,
		}, nil
	case "queue":
		queue, err := s.GetTaskGenerationQueue(ctx, taskID, cloneGenerationQueueQuery(target.QueueQuery))
		if err != nil {
			return nil, err
		}
		return &GenerationReviewNavigationDispatchResponse{
			TaskID:       taskID,
			DispatchKind: "queue",
			ResponseMode: responseMode,
			DeltaToken:   queue.DeltaToken,
			Queue:        queue,
		}, nil
	default:
		sessionQuery := cloneGenerationQueueQuery(target.SessionQuery)
		if sessionQuery == nil {
			sessionQuery = cloneGenerationQueueQuery(target.QueueQuery)
		}
		if sessionQuery == nil {
			sessionQuery = cloneGenerationQueueQuery(target.PreviewQuery)
		}
		if sessionQuery != nil && strings.TrimSpace(responseMode) != "" {
			sessionQuery.ResponseMode = responseMode
		}
		session, err := s.GetTaskGenerationReviewSession(ctx, taskID, sessionQuery)
		if err != nil {
			return nil, err
		}
		return &GenerationReviewNavigationDispatchResponse{
			TaskID:        taskID,
			DispatchKind:  "session",
			ResponseMode:  responseMode,
			DeltaToken:    session.DeltaToken,
			ReviewSession: session,
		}, nil
	}
}

func (s *taskGenerationService) executeGenerationNavigationDispatchPlan(ctx context.Context, taskID string, target *GenerationReviewNavigationTarget, responseMode string) (*GenerationNavigationDispatchExecution, error) {
	if target == nil || target.Descriptor == nil || target.Descriptor.DispatchPlan == nil {
		return nil, nil
	}
	plan := cloneGenerationNavigationDispatchPlan(target.Descriptor.DispatchPlan)
	if plan == nil {
		return nil, nil
	}
	execution := &GenerationNavigationDispatchExecution{
		Strategy: plan.Strategy,
		Steps:    make([]GenerationNavigationDispatchExecutionStep, 0, len(plan.Steps)),
	}
	if generationNavigationDispatchPlanRunsInParallel(plan) {
		s.executeGenerationNavigationDispatchPlanParallel(ctx, taskID, responseMode, plan, execution)
		applyGenerationNavigationDispatchExecutionRules(plan, execution)
		return execution, nil
	}
	s.executeGenerationNavigationDispatchPlanSequential(ctx, taskID, responseMode, plan, execution)
	applyGenerationNavigationDispatchExecutionRules(plan, execution)
	return execution, nil
}

func (s *taskGenerationService) executeGenerationNavigationDispatchPlanSequential(ctx context.Context, taskID string, responseMode string, plan *GenerationNavigationDispatchPlan, execution *GenerationNavigationDispatchExecution) {
	for index, step := range plan.Steps {
		stepResult := s.executeGenerationNavigationDispatchPlanStep(ctx, taskID, step, responseMode)
		execution.Steps = append(execution.Steps, *stepResult)
		applyGenerationNavigationDispatchExecutionStats(execution, stepResult)
		if stepResult.Status == "failed" && plan.StopOnError {
			execution.StopReason = "error"
		}
		if execution.StopReason == "" && shouldStopGenerationNavigationDispatchPlan(plan, stepResult) {
			execution.StopReason = generationNavigationDispatchPlanStopReason(plan, stepResult)
		}
		if execution.StopReason != "" {
			for remaining := index + 1; remaining < len(plan.Steps); remaining++ {
				next := plan.Steps[remaining]
				skipped := generationNavigationDispatchPlanSkippedStep(next, execution.StopReason)
				execution.Steps = append(execution.Steps, skipped)
				applyGenerationNavigationDispatchExecutionStats(execution, &skipped)
			}
			break
		}
	}
}

func (s *taskGenerationService) executeGenerationNavigationDispatchPlanParallel(ctx context.Context, taskID string, responseMode string, plan *GenerationNavigationDispatchPlan, execution *GenerationNavigationDispatchExecution) {
	type dedupeEntry struct {
		step   GenerationNavigationDispatchStep
		result *GenerationNavigationDispatchExecutionStep
	}
	entries := make([]dedupeEntry, 0, len(plan.Steps))
	indexByKey := make(map[string]int, len(plan.Steps))
	for _, step := range plan.Steps {
		key := generationNavigationDispatchStepDeduplicationKey(step, responseMode)
		if existing, ok := indexByKey[key]; ok {
			deduped := generationNavigationDispatchPlanDeduplicatedStep(step, key, existing)
			entries = append(entries, dedupeEntry{step: step, result: &deduped})
			continue
		}
		result := generationNavigationDispatchExecutionPendingStep(step, key, responseMode)
		indexByKey[key] = len(entries)
		entries = append(entries, dedupeEntry{step: step, result: result})
	}
	maxParallelism := plan.MaxParallelism
	if maxParallelism <= 0 {
		maxParallelism = 1
	}
	sem := make(chan struct{}, maxParallelism)
	var wg sync.WaitGroup
	for index := range entries {
		if entries[index].result.Status == "deduplicated" {
			continue
		}
		wg.Add(1)
		go func(entry *dedupeEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			entry.result = s.executeGenerationNavigationDispatchPlanStep(ctx, taskID, entry.step, responseMode)
			entry.result.DeduplicationKey = generationNavigationDispatchStepDeduplicationKey(entry.step, responseMode)
		}(&entries[index])
	}
	wg.Wait()
	for _, entry := range entries {
		stepResult := entry.result
		if stepResult == nil {
			continue
		}
		if stepResult.Status == "deduplicated" {
			if source := entry.result.DeduplicatedFrom; source >= 0 && source < len(entries) && entries[source].result != nil {
				stepResult.DeltaToken = entries[source].result.DeltaToken
				stepResult.NotModified = entries[source].result.NotModified
				stepResult.NoChanges = entries[source].result.NoChanges
			}
		}
		execution.Steps = append(execution.Steps, *stepResult)
		applyGenerationNavigationDispatchExecutionStats(execution, stepResult)
	}
}

func (s *taskGenerationService) executeGenerationNavigationDispatchPlanStep(ctx context.Context, taskID string, step GenerationNavigationDispatchStep, responseMode string) *GenerationNavigationDispatchExecutionStep {
	result := &GenerationNavigationDispatchExecutionStep{
		Kind:               step.Kind,
		ResponseMode:       firstNonEmpty(step.ResponseMode, responseMode),
		CachePreference:    step.CachePreference,
		RequiresRevalidate: step.RequiresRevalidate,
		DeduplicationKey:   generationNavigationDispatchStepDeduplicationKey(step, responseMode),
		Executed:           true,
		Status:             "completed",
	}
	query := cloneGenerationQueueQuery(step.Query)
	if query != nil && strings.TrimSpace(result.ResponseMode) != "" {
		query.ResponseMode = result.ResponseMode
	}
	switch strings.ToLower(strings.TrimSpace(step.Kind)) {
	case "queue":
		queue, err := s.GetTaskGenerationQueue(ctx, taskID, query)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			result.ErrorKind = classifyGenerationNavigationDispatchStepError(err)
			return result
		}
		result.Queue = queue
		if queue != nil {
			result.DeltaToken = queue.DeltaToken
			result.NotModified = queue.NotModified
			result.NoChanges = queue.NotModified
			if queue.NotModified {
				result.Status = "not_modified"
			}
		}
	case "preview":
		preview, err := s.GetTaskGenerationReviewPreview(ctx, taskID, query)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			result.ErrorKind = classifyGenerationNavigationDispatchStepError(err)
			return result
		}
		result.ReviewPreview = preview
		if preview != nil {
			result.DeltaToken = preview.DeltaToken
			result.NotModified = preview.NotModified
			result.NoChanges = preview.NotModified
			if preview.NotModified {
				result.Status = "not_modified"
			}
		}
	default:
		session, err := s.GetTaskGenerationReviewSession(ctx, taskID, query)
		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			result.ErrorKind = classifyGenerationNavigationDispatchStepError(err)
			return result
		}
		result.ReviewSession = session
		if session != nil {
			result.DeltaToken = session.DeltaToken
			result.NotModified = session.NotModified
			result.NoChanges = session.NotModified
			if session.NotModified {
				result.Status = "not_modified"
			}
		}
	}
	return result
}

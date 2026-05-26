package listingkit

import (
	"context"

	assetgeneration "task-processor/internal/asset/generation"
)

type taskGenerationServiceConfig struct {
	repo                           Repository
	listAssetGenerationTasks       func(context.Context, string) ([]assetgeneration.Task, error)
	listGenerationReviews          func(context.Context, string) ([]GenerationReviewRecord, error)
	getCurrentListingKitResult     func(context.Context, string) (*ListingKitResult, error)
	getCurrentAssetGenerationQueue func(context.Context, string) (*GenerationWorkQueue, error)
}

type taskGenerationService struct {
	repo                           Repository
	listAssetGenerationTasks       func(context.Context, string) ([]assetgeneration.Task, error)
	listGenerationReviews          func(context.Context, string) ([]GenerationReviewRecord, error)
	getCurrentListingKitResult     func(context.Context, string) (*ListingKitResult, error)
	getCurrentAssetGenerationQueue func(context.Context, string) (*GenerationWorkQueue, error)
}

func newTaskGenerationService(config taskGenerationServiceConfig) *taskGenerationService {
	return &taskGenerationService{
		repo:                           config.repo,
		listAssetGenerationTasks:       config.listAssetGenerationTasks,
		listGenerationReviews:          config.listGenerationReviews,
		getCurrentListingKitResult:     config.getCurrentListingKitResult,
		getCurrentAssetGenerationQueue: config.getCurrentAssetGenerationQueue,
	}
}

func (s *taskGenerationService) GetTaskGenerationTasks(ctx context.Context, taskID string, query *GenerationTaskQuery) (*GenerationTaskPage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	rawTasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	tasks := rawTasks
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
	rawTasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	tasks := rawTasks
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

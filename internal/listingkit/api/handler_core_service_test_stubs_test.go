package api

import (
	"context"
	"errors"

	"task-processor/internal/listingkit"
)

type stubHandlerCoreService struct {
	stubTaskLifecycleHandlerService
}

var _ HandlerService = (*stubHandlerCoreService)(nil)

func (stubHandlerCoreService) GetTaskGenerationTasks(context.Context, string, *listingkit.GenerationTaskQuery) (*listingkit.GenerationTaskPage, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) GetTaskGenerationQueue(context.Context, string, *listingkit.GenerationQueueQuery) (*listingkit.GenerationQueuePage, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) GetTaskGenerationReviewSession(context.Context, string, *listingkit.GenerationQueueQuery) (*listingkit.GenerationReviewSessionResponse, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) GetTaskGenerationReviewPreview(context.Context, string, *listingkit.GenerationQueueQuery) (*listingkit.GenerationReviewPreviewResponse, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) DispatchTaskGenerationNavigation(context.Context, string, *listingkit.GenerationReviewNavigationDispatchRequest) (*listingkit.GenerationReviewNavigationDispatchResponse, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) RetryTaskGenerationTasks(context.Context, string, *listingkit.RetryGenerationTasksRequest) (*listingkit.GenerationTaskPage, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) ExecuteTaskGenerationAction(context.Context, string, *listingkit.ExecuteGenerationActionRequest) (*listingkit.GenerationActionExecutionResult, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) UploadImages(context.Context, *listingkit.UploadImagesRequest) (*listingkit.UploadImagesResponse, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) GetUploadedImage(context.Context, string) (*listingkit.UploadedImageFile, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) AnalyzeStudioReferenceStyle(context.Context, *listingkit.StudioReferenceAnalysisRequest) (*listingkit.StudioReferenceAnalysisResponse, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) GenerateStudioDesigns(context.Context, *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) GenerateStudioProductImages(context.Context, *listingkit.StudioProductImageRequest) (*listingkit.StudioProductImageResponse, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) RegenerateSheinDataImage(context.Context, string, *listingkit.RegenerateSheinDataImageRequest) (*listingkit.RegenerateSheinDataImageResponse, error) {
	return nil, errors.New("not implemented")
}

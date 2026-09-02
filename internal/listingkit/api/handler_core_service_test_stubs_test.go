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

func (stubHandlerCoreService) UploadImages(context.Context, *listingkit.UploadImagesRequest) (*listingkit.UploadImagesResponse, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) GetUploadedImage(context.Context, string) (*listingkit.UploadedImageFile, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) DeleteUploadedImage(context.Context, string) (*listingkit.DeletedUploadedImage, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) AnalyzeStudioReferenceStyle(context.Context, *listingkit.StudioReferenceAnalysisRequest) (*listingkit.StudioReferenceAnalysisResponse, error) {
	return nil, errors.New("not implemented")
}

func (stubHandlerCoreService) GenerateStudioDesigns(context.Context, *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	return nil, errors.New("not implemented")
}

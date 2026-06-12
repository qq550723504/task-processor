package api

import (
	"context"
	"errors"
	"strings"

	"task-processor/internal/listingkit"
)

type stubStudioMediaHandlerService struct {
	uploadResponse          *listingkit.UploadImagesResponse
	uploadedImageFile       *listingkit.UploadedImageFile
	deletedUploadedImage    *listingkit.DeletedUploadedImage
	studioDesigns           *listingkit.StudioDesignResponse
	studioProductImages     *listingkit.StudioProductImageResponse
	err                     error
	uploadedImageKey        string
	deletedUploadedImageKey string
	uploadImagesReq         *listingkit.UploadImagesRequest
	studioDesignCtx         context.Context
	studioDesignReq         *listingkit.StudioDesignRequest
	studioProductImageCtx   context.Context
	studioProductImageReq   *listingkit.StudioProductImageRequest
	updatedStudioSessionCtx context.Context
	updatedStudioSessionID  string
	updatedStudioSessionReq *listingkit.UpdateStudioSessionRequest
}

func (s *stubStudioMediaHandlerService) UploadImages(_ context.Context, req *listingkit.UploadImagesRequest) (*listingkit.UploadImagesResponse, error) {
	s.uploadImagesReq = req
	if s.uploadResponse != nil || s.err != nil {
		return s.uploadResponse, s.err
	}
	return nil, errors.New("not implemented")
}

func (s *stubStudioMediaHandlerService) GetUploadedImage(_ context.Context, key string) (*listingkit.UploadedImageFile, error) {
	s.uploadedImageKey = key
	if s.uploadedImageFile != nil || s.err != nil {
		return s.uploadedImageFile, s.err
	}
	return nil, errors.New("not implemented")
}

func (s *stubStudioMediaHandlerService) DeleteUploadedImage(_ context.Context, key string) (*listingkit.DeletedUploadedImage, error) {
	s.deletedUploadedImageKey = key
	if s.deletedUploadedImage != nil || s.err != nil {
		return s.deletedUploadedImage, s.err
	}
	return nil, errors.New("not implemented")
}

func (s *stubStudioMediaHandlerService) GenerateStudioDesigns(ctx context.Context, req *listingkit.StudioDesignRequest) (*listingkit.StudioDesignResponse, error) {
	s.studioDesignCtx = ctx
	s.studioDesignReq = req
	return s.studioDesigns, s.err
}

func (s *stubStudioMediaHandlerService) GenerateStudioProductImages(ctx context.Context, req *listingkit.StudioProductImageRequest) (*listingkit.StudioProductImageResponse, error) {
	s.studioProductImageCtx = ctx
	s.studioProductImageReq = req
	return s.studioProductImages, s.err
}

func (s *stubStudioMediaHandlerService) RegenerateSheinDataImage(context.Context, string, *listingkit.RegenerateSheinDataImageRequest) (*listingkit.RegenerateSheinDataImageResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubStudioMediaHandlerService) SyncStudioDesignAsyncJob(ctx context.Context, sessionID string, jobStatus listingkit.StudioAsyncJobStatus, jobID string, errMessage string) error {
	s.updatedStudioSessionCtx = ctx
	s.updatedStudioSessionID = sessionID
	sessionStatus := listingkit.SheinStudioSessionStatusGenerating
	switch jobStatus {
	case listingkit.StudioAsyncJobStatusSucceeded:
		sessionStatus = listingkit.SheinStudioSessionStatusGenerated
	case listingkit.StudioAsyncJobStatusFailed:
		sessionStatus = listingkit.SheinStudioSessionStatusFailed
	}
	trimmedJobID := strings.TrimSpace(jobID)
	trimmedErr := strings.TrimSpace(errMessage)
	s.updatedStudioSessionReq = &listingkit.UpdateStudioSessionRequest{
		Status:          &sessionStatus,
		GenerationJobID: &trimmedJobID,
		GenerationError: &trimmedErr,
	}
	return s.err
}

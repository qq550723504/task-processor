package listingkit

import (
	"context"
	"fmt"
	"strings"
)

func (s *service) GetStudioBatchDetail(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().GetStudioBatchDetail(ctx, batchID)
}

func (s *service) StartStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().StartStudioBatchGeneration(ctx, batchID)
}

func (s *service) PrepareStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().PrepareStudioBatchGeneration(ctx, batchID)
}

func (s *service) ResumeStudioBatchGeneration(ctx context.Context, batchID string) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().ResumeStudioBatchGeneration(ctx, batchID)
}

func (s *service) PrepareRetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().PrepareRetryStudioBatchItems(ctx, batchID, req)
}

func (s *service) RetryStudioBatchItems(ctx context.Context, batchID string, req *RetryStudioBatchItemsRequest) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().RetryStudioBatchItems(ctx, batchID, req)
}

func (s *service) RetryStudioBatchDesignBackgroundRemoval(ctx context.Context, batchID string, req *RetryStudioBatchDesignBackgroundRemovalRequest) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().RetryStudioBatchDesignBackgroundRemoval(ctx, batchID, req)
}

func (s *service) ApplyManualStudioBatchDesignBackgroundRemoval(ctx context.Context, batchID string, designID string, input *ImageUploadInput) (*StudioBatchDetail, error) {
	if input == nil {
		return nil, NewStudioBatchActionValidationError("manual background removal file is required")
	}
	validated, err := validateUploadedImage(*input)
	if err != nil {
		return nil, fmt.Errorf("invalid manual background removal image: %w", err)
	}
	if validated.ContentType != "image/png" {
		return nil, NewStudioBatchActionValidationError("manual background removal upload must be a PNG image")
	}

	upload, err := s.UploadImages(ctx, &UploadImagesRequest{Files: []ImageUploadInput{{
		Filename:    input.Filename,
		ContentType: validated.ContentType,
		Data:        append([]byte(nil), input.Data...),
	}}})
	if err != nil {
		return nil, err
	}
	if upload == nil || len(upload.ImageURLs) == 0 || strings.TrimSpace(upload.ImageURLs[0]) == "" {
		return nil, fmt.Errorf("manual background removal upload returned no image url")
	}

	imageURL := strings.TrimSpace(upload.ImageURLs[0])
	detail, applyErr := s.taskStudioBatchOrDefault().ApplyManualStudioBatchDesignBackgroundRemoval(ctx, batchID, designID, imageURL)
	if applyErr == nil {
		return detail, nil
	}
	uploadID := strings.TrimPrefix(imageURL, buildUploadedImagePath(""))
	if uploadID != "" {
		_, _ = s.DeleteUploadedImage(ctx, uploadID)
	}
	return nil, applyErr
}

func (s *service) ApproveStudioBatchDesigns(ctx context.Context, batchID string, req *ApproveStudioBatchDesignsRequest) (*StudioBatchDetail, error) {
	return s.taskStudioBatchOrDefault().ApproveStudioBatchDesigns(ctx, batchID, req)
}

func (s *service) CreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {
	return s.taskStudioBatchOrDefault().CreateStudioBatchTasks(ctx, batchID, req)
}

func (s *service) PrepareCreateStudioBatchTasks(ctx context.Context, batchID string, req *CreateStudioBatchTasksRequest) (*CreateStudioBatchTasksResult, error) {
	return s.taskStudioBatchOrDefault().PrepareCreateStudioBatchTasks(ctx, batchID, req)
}

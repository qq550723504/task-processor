package listingkit

import (
	"context"
	"errors"
	"fmt"
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
		if errors.Is(err, errInvalidUploadedImage) {
			return nil, NewStudioBatchActionValidationError("manual background removal upload must be a valid image")
		}
		return nil, fmt.Errorf("invalid manual background removal image: %w", err)
	}
	if validated.ContentType != "image/png" {
		return nil, NewStudioBatchActionValidationError("manual background removal upload must be a PNG image")
	}
	batchService := s.taskStudioBatchOrDefault()
	var preflightTarget *StudioMaterializedDesignRecord
	if batchService != nil && batchService.repo != nil {
		if _, preflightTarget, err = batchService.validateManualStudioBatchDesignBackgroundRemoval(ctx, batchID, designID); err != nil {
			return nil, err
		}
	}

	materialized, err := s.uploadListingKitImage(ctx, ImageUploadInput{
		Filename:    input.Filename,
		ContentType: validated.ContentType,
		Data:        append([]byte(nil), input.Data...),
	})
	if err != nil {
		return nil, err
	}
	if materialized == nil || materialized.imageURL == "" {
		return nil, fmt.Errorf("manual background removal upload returned no image url")
	}

	var detail *StudioBatchDetail
	var applyErr error
	if preflightTarget != nil {
		detail, applyErr = batchService.applyManualStudioBatchDesignBackgroundRemovalTarget(ctx, batchID, preflightTarget, materialized.imageURL)
	} else {
		detail, applyErr = s.taskStudioBatchOrDefault().ApplyManualStudioBatchDesignBackgroundRemoval(ctx, batchID, designID, materialized.imageURL)
	}
	if applyErr == nil {
		return detail, nil
	}
	var committedErr *studioManualBackgroundRemovalCommittedError
	if materialized.cleanup != nil && !errors.As(applyErr, &committedErr) {
		cleanupErr := materialized.cleanup(context.WithoutCancel(ctx))
		if cleanupErr != nil && !errors.Is(cleanupErr, ErrUploadedImageNotFound) {
			return nil, errors.Join(applyErr, fmt.Errorf("cleanup manual background removal upload: %w", cleanupErr))
		}
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

package listingkit

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func (s *service) RegenerateSheinDataImage(ctx context.Context, taskID string, req *RegenerateSheinDataImageRequest) (*RegenerateSheinDataImageResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid request: request is required")
	}
	oldURL := strings.TrimSpace(req.ImageURL)
	if oldURL == "" {
		return nil, fmt.Errorf("invalid request: image_url is required")
	}
	fixPrompt := strings.TrimSpace(req.Prompt)
	if fixPrompt == "" {
		return nil, fmt.Errorf("invalid request: prompt is required")
	}

	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil || task.Result.Shein == nil {
		return nil, ErrTaskResultUnavailable
	}
	if s.studioImageGenerator == nil {
		return nil, fmt.Errorf("studio image generator is not configured")
	}

	productReq, role := buildSheinDataImageRegenerationRequest(task, req)
	sourceURL := strings.TrimSpace(productReq.SourceDesignURL)
	if sourceURL == "" {
		sourceURL = oldURL
		productReq.SourceDesignURL = sourceURL
	}
	promptText := buildStudioProductImagePrompt(productReq, role, 1, 1)
	newURL, err := s.generateOneStudioProductImage(ctx, productReq, sourceURL, promptText)
	if err != nil {
		return nil, err
	}

	replaced := replaceSheinDataImageURL(task, oldURL, newURL)
	if replaced == 0 {
		return nil, fmt.Errorf("invalid request: image_url was not found in this SHEIN task")
	}
	if err := s.repo.SaveTaskResult(ctx, task.ID, task.Result); err != nil {
		return nil, err
	}

	preview, err := s.GetTaskPreview(ctx, task.ID, "shein")
	if err != nil {
		return nil, err
	}
	return &RegenerateSheinDataImageResponse{
		Preview: preview,
		Image: StudioGeneratedImage{
			ID:            uuid.NewString(),
			ImageURL:      newURL,
			RevisedPrompt: fixPrompt,
			Role:          role.Key,
			RoleLabel:     role.Label,
		},
		ReplacedURL: oldURL,
	}, nil
}

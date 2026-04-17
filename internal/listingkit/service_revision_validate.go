package listingkit

import (
	"context"
	"strings"
)

func (s *service) ValidateTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*RevisionValidationResult, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	platform := ""
	if req != nil {
		platform = strings.ToLower(strings.TrimSpace(req.Platform))
	}
	effectiveReq, restorePreview, err := resolveRevisionValidationRequest(task.Result, req)
	if err != nil {
		return nil, err
	}
	if effectiveReq != nil {
		platform = strings.ToLower(strings.TrimSpace(effectiveReq.Platform))
	}
	validationErr := validateApplyRevisionRequest(effectiveReq)
	return buildRevisionValidationResult(taskID, platform, task.Result, validationErr, restorePreview), nil
}

package listingkit

import (
	"context"
	"errors"
	"fmt"
)

func (s *service) ProcessListingKit(ctx context.Context, task *Task) (*ListingKitResult, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}
	if err := s.repo.MarkProcessing(ctx, task.ID); err != nil {
		if errors.Is(err, ErrTaskNotPending) {
			return nil, ErrTaskNotPending
		}
		return nil, fmt.Errorf("failed to mark task as processing: %w", err)
	}

	result, err := s.runWorkflow(ctx, task)
	if err != nil {
		if result != nil {
			_ = s.repo.SaveTaskResult(ctx, task.ID, result)
		}
		_ = s.repo.MarkFailed(ctx, task.ID, err.Error())
		return nil, err
	}

	result.Status = string(TaskStatusCompleted)
	if err := s.repo.MarkCompleted(ctx, task.ID, result); err != nil {
		return nil, err
	}
	return result, nil
}

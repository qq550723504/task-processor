package productimage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *service) CreateProcessTask(ctx context.Context, req *ImageProcessRequest) (*Task, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if err := s.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	task := &Task{
		ID:         uuid.New().String(),
		Request:    req,
		Status:     TaskStatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RetryCount: 0,
	}

	if err := s.taskRepo.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	if s.taskSubmitter != nil {
		if err := s.taskSubmitter.Submit(task.ID); err != nil {
			_ = s.taskRepo.UpdateTaskError(ctx, task.ID, fmt.Sprintf("failed to submit task: %v", err))
			return nil, fmt.Errorf("failed to submit task: %w", err)
		}
	}

	return task, nil
}

func (s *service) GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID cannot be empty")
	}

	task, err := s.taskRepo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	result := &TaskResult{
		TaskID:    task.ID,
		Status:    task.Status,
		Result:    task.Result,
		Error:     task.Error,
		CreatedAt: task.CreatedAt,
	}
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusNeedsReview || task.Status == TaskStatusRejected || task.Status == TaskStatusFailed {
		result.CompletedAt = &task.UpdatedAt
	}
	return result, nil
}

func (s *service) ReviewTask(ctx context.Context, taskID string, req *ReviewTaskRequest) (*TaskResult, error) {
	if taskID == "" {
		return nil, fmt.Errorf("task ID cannot be empty")
	}
	if req == nil {
		return nil, fmt.Errorf("review request cannot be nil")
	}

	task, err := s.taskRepo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	switch action {
	case "approve":
		if task.Status != TaskStatusNeedsReview {
			return nil, fmt.Errorf("invalid request: only needs_review tasks can be approved")
		}
		if task.Result == nil {
			return nil, fmt.Errorf("invalid request: task result is required for approval")
		}
		if err := s.taskRepo.MarkCompleted(ctx, taskID, task.Result); err != nil {
			return nil, fmt.Errorf("approve review task: %w", err)
		}
	case "reject":
		if task.Status != TaskStatusNeedsReview {
			return nil, fmt.Errorf("invalid request: only needs_review tasks can be rejected")
		}
		reason := strings.TrimSpace(req.Reason)
		if reason == "" {
			reason = "rejected during manual review"
		}
		if err := s.taskRepo.MarkRejected(ctx, taskID, reason); err != nil {
			return nil, fmt.Errorf("reject review task: %w", err)
		}
	case "retry":
		if task.Status != TaskStatusNeedsReview && task.Status != TaskStatusRejected && task.Status != TaskStatusFailed {
			return nil, fmt.Errorf("invalid request: only needs_review, rejected, or failed tasks can be retried")
		}
		if err := s.taskRepo.PrepareRetry(ctx, taskID); err != nil {
			return nil, fmt.Errorf("prepare review retry: %w", err)
		}
		if s.taskSubmitter != nil {
			if err := s.taskSubmitter.Submit(taskID); err != nil {
				_ = s.taskRepo.UpdateTaskError(ctx, taskID, fmt.Sprintf("failed to resubmit task: %v", err))
				return nil, fmt.Errorf("resubmit review task: %w", err)
			}
		}
	default:
		return nil, fmt.Errorf("invalid request: unsupported review action %q", req.Action)
	}

	return s.GetTaskResult(ctx, taskID)
}

func (s *service) validateRequest(req *ImageProcessRequest) error {
	if len(req.ImageURLs) == 0 && req.ProductURL == "" {
		return fmt.Errorf("at least one of image_urls or product_url must be provided")
	}
	if len(req.ImageURLs) > 20 {
		return fmt.Errorf("too many image URLs (max 20)")
	}
	if strings.TrimSpace(req.Marketplace) == "" {
		return fmt.Errorf("marketplace is required")
	}
	if !strings.EqualFold(req.Marketplace, "amazon") {
		return fmt.Errorf("only amazon marketplace is supported currently")
	}
	return nil
}

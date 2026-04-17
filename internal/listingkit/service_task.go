package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *service) CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	normalizeGenerateRequest(req)
	if err := validateRequest(req); err != nil {
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
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}
	if s.taskSubmitter != nil {
		if err := s.taskSubmitter.Submit(task.ID); err != nil {
			_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to submit task: %v", err))
			return nil, fmt.Errorf("failed to submit task: %w", err)
		}
	}
	return task, nil
}

func (s *service) GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error) {
	task, err := s.repo.GetTask(ctx, taskID)
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
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
		result.CompletedAt = &task.UpdatedAt
	}
	return result, nil
}

func validateRequest(req *GenerateRequest) error {
	if len(req.ImageURLs) == 0 && strings.TrimSpace(req.Text) == "" && strings.TrimSpace(req.ProductURL) == "" {
		return fmt.Errorf("at least one of image_urls, text, or product_url must be provided")
	}
	if len(req.ImageURLs) > 10 {
		return fmt.Errorf("too many image URLs (max 10)")
	}
	if len(req.Platforms) == 0 {
		return fmt.Errorf("at least one platform is required")
	}
	return nil
}

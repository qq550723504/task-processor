package amazonlisting

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
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed || task.Status == TaskStatusRejected {
		result.CompletedAt = &task.UpdatedAt
	}
	return result, nil
}

func (s *service) ReviewTask(ctx context.Context, taskID string, req *ReviewTaskRequest) (*TaskResult, error) {
	if req == nil {
		return nil, fmt.Errorf("review request cannot be nil")
	}
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(strings.TrimSpace(req.Action)) {
	case "approve":
		if task.Result == nil {
			return nil, fmt.Errorf("task result is empty")
		}
		task.Result.Status = string(TaskStatusCompleted)
		if task.Result.Review == nil {
			task.Result.Review = &AmazonReviewReport{}
		}
		task.Result.Review.NeedsReview = false
		task.Result.Review.Reasons = nil
		if task.Result.Compliance == nil {
			task.Result.Compliance = &AmazonComplianceReport{}
		}
		task.Result.Compliance.Ready = true
		if err := s.repo.MarkCompleted(ctx, taskID, task.Result); err != nil {
			return nil, err
		}
	case "reject":
		if err := s.repo.MarkRejected(ctx, taskID, req.Reason); err != nil {
			return nil, err
		}
	case "retry":
		if err := s.repo.PrepareRetry(ctx, taskID); err != nil {
			return nil, err
		}
		if s.taskSubmitter != nil {
			if err := s.taskSubmitter.Submit(taskID); err != nil {
				_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to resubmit task: %v", err))
				return nil, err
			}
		}
	case "apply_edits":
		if task.Result == nil {
			return nil, fmt.Errorf("task result is empty")
		}
		if len(req.Edits) == 0 {
			return nil, fmt.Errorf("edit request is empty")
		}
		ensureCanonicalProduct(task)
		if err := applyCanonicalEdits(task.Result.CanonicalProduct, req.Edits); err != nil {
			return nil, err
		}
		syncDraftFromCanonical(task.Result, task.Result.CanonicalProduct)
		if err := applyDraftEdits(task.Result, req.Edits); err != nil {
			return nil, err
		}
		task.Result.ReviewItems = refreshCanonicalReviewItems(removeResolvedReviewItems(task.Result.ReviewItems, req.Edits), task.Result.CanonicalProduct)
		if s.exportBuilder != nil {
			task.Result.Export = s.exportBuilder.Build(task.Request, task.Result)
		}
		report := s.validator.Validate(task.Request, task.Result)
		task.Result.Compliance = &AmazonComplianceReport{
			Ready:          report.Ready,
			BlockingIssues: append([]string(nil), report.BlockingIssues...),
			Warnings:       append([]string(nil), report.Warnings...),
		}
		task.Result.Review = &AmazonReviewReport{
			NeedsReview: report.NeedsReview,
			Reasons:     append([]string(nil), report.ReviewReasons...),
		}
		task.Result.Status = string(TaskStatusNeedsReview)
		if len(report.BlockingIssues) == 0 && !report.NeedsReview {
			task.Result.Status = string(TaskStatusCompleted)
			if err := s.repo.MarkCompleted(ctx, taskID, task.Result); err != nil {
				return nil, err
			}
		} else {
			reason := strings.Join(report.ReviewReasons, "; ")
			if len(report.BlockingIssues) > 0 {
				reason = strings.Join(report.BlockingIssues, "; ")
			}
			if err := s.repo.MarkNeedsReview(ctx, taskID, task.Result, reason); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("unsupported review action: %s", req.Action)
	}

	return s.GetTaskResult(ctx, taskID)
}

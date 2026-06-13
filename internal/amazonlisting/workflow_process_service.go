package amazonlisting

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (s *service) ProcessListing(ctx context.Context, task *Task) (*AmazonListingDraft, error) {
	if task == nil {
		return nil, fmt.Errorf("task cannot be nil")
	}
	if err := s.repo.MarkProcessing(ctx, task.ID); err != nil {
		if errors.Is(err, ErrTaskNotPending) {
			return nil, ErrTaskNotPending
		}
		return nil, fmt.Errorf("failed to mark task as processing: %w", err)
	}

	artifacts, err := s.workflow.Run(ctx, task)
	if err != nil {
		var workflowErr *WorkflowError
		if errors.As(err, &workflowErr) && workflowErr.Artifacts != nil && workflowErr.Artifacts.Draft != nil {
			_ = s.repo.SaveTaskResult(ctx, task.ID, workflowErr.Artifacts.Draft)
		}
		_ = s.repo.MarkFailed(ctx, task.ID, err.Error())
		return nil, err
	}

	draft := artifacts.Draft

	report := s.validator.Validate(task.Request, draft)
	draft.Compliance = &AmazonComplianceReport{
		Ready:          report.Ready,
		BlockingIssues: append([]string(nil), report.BlockingIssues...),
		Warnings:       append([]string(nil), report.Warnings...),
	}
	draft.Review = &AmazonReviewReport{
		NeedsReview: report.NeedsReview,
		Reasons:     append([]string(nil), report.ReviewReasons...),
	}

	if len(report.BlockingIssues) > 0 {
		draft.Status = string(TaskStatusFailed)
		if err := s.repo.MarkFailed(ctx, task.ID, strings.Join(report.BlockingIssues, "; ")); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%s", strings.Join(report.BlockingIssues, "; "))
	}
	if report.NeedsReview {
		draft.Status = string(TaskStatusNeedsReview)
		if err := s.repo.MarkNeedsReview(ctx, task.ID, draft, strings.Join(report.ReviewReasons, "; ")); err != nil {
			return nil, err
		}
		return draft, nil
	}
	draft.Status = string(TaskStatusCompleted)
	if err := s.repo.MarkCompleted(ctx, task.ID, draft); err != nil {
		return nil, err
	}
	return draft, nil
}

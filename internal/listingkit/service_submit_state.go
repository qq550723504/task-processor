package listingkit

import (
	"context"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) taskSubmissionStateOrDefault() *taskSubmissionStateService {
	if s.taskSubmissionState != nil {
		return s.taskSubmissionState
	}
	s.taskSubmissionState = newTaskSubmissionStateService(taskSubmissionStateServiceConfig{
		repo:                   s.repo,
		rememberSheinSubmitted: s.rememberSheinSubmittedResolution,
	})
	return s.taskSubmissionState
}

func (s *service) persistSuccessfulSheinSubmission(ctx context.Context, taskID string, task *Task, action string) error {
	return s.taskSubmissionStateOrDefault().persistSuccessfulSheinSubmission(ctx, taskID, task, action)
}

func sheinSubmissionSucceeded(action string, pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.SubmissionState == nil {
		return false
	}
	record := sheinSubmissionRecordForAction(pkg.SubmissionState, action)
	return record != nil && record.Status == sheinpub.SubmissionStatusSuccess
}

func applySuccessfulSheinSubmissionState(task *Task) {
	if task == nil {
		return
	}
	task.Status = TaskStatusCompleted
	task.Error = ""
	if task.Result == nil {
		return
	}
	task.Result.Status = string(TaskStatusCompleted)
	task.Result.ReviewReasons = nil
	clearResolvedSheinReviewState(task.Result)
	task.Result.UpdatedAt = time.Now()
}

func clearResolvedSheinReviewState(result *ListingKitResult) {
	if result == nil {
		return
	}
	if len(result.WorkflowIssues) > 0 {
		filtered := result.WorkflowIssues[:0]
		for _, issue := range result.WorkflowIssues {
			if issue.Stage == "shein_review" {
				continue
			}
			filtered = append(filtered, issue)
		}
		result.WorkflowIssues = filtered
	}
	if result.Summary != nil {
		result.Summary.NeedsReview = false
	}
	newWorkflowRecorder(result).FinalizeSummary()
}

func (s *service) persistSheinDirectSubmitPhase(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions, phase string) error {
	return s.taskSubmissionStateOrDefault().persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, phase)
}

func (s *service) persistSheinSubmitPhase(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID, phase string) error {
	return s.taskSubmissionStateOrDefault().persistSheinSubmitPhase(ctx, taskID, result, pkg, action, requestID, phase)
}

func (s *service) persistSuccessfulSheinDirectResponse(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions, supplierCode string, response *sheinpub.SubmissionResponse) error {
	return s.taskSubmissionStateOrDefault().persistSuccessfulSheinDirectResponse(ctx, taskID, task, pkg, opts, supplierCode, response)
}

func (s *service) finishSheinDirectSubmitAttempt(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions, response *sheinpub.SubmissionResponse, responseErr error) error {
	return s.taskSubmissionStateOrDefault().finishSheinDirectSubmitAttempt(ctx, taskID, task, pkg, opts, response, responseErr)
}

func (s *service) recordSheinSubmissionFailure(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action string, submitErr error) error {
	return s.taskSubmissionStateOrDefault().recordSheinSubmissionFailure(ctx, taskID, result, pkg, action, submitErr)
}

func (s *service) recordSheinSubmissionFailureForState(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestedID, phase string, submitErr error) error {
	return s.taskSubmissionStateOrDefault().recordSheinSubmissionFailureForState(ctx, taskID, result, pkg, action, requestedID, phase, submitErr)
}

func (s *service) failSheinDirectSubmit(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, action string, submitErr error) error {
	return s.taskSubmissionStateOrDefault().failSheinDirectSubmit(ctx, taskID, task, pkg, action, submitErr)
}

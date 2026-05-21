package listingkit

import (
	"context"
	"strings"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) persistSuccessfulSheinSubmission(ctx context.Context, taskID string, task *Task, action string) error {
	if task == nil || task.Result == nil {
		return nil
	}
	if success := sheinSubmissionSucceeded(action, task.Result.Shein); success {
		applySuccessfulSheinSubmissionState(task)
		return s.repo.MarkCompleted(ctx, taskID, task.Result)
	}
	task.Result.UpdatedAt = time.Now()
	return s.repo.SaveTaskResult(ctx, taskID, task.Result)
}

func sheinSubmissionSucceeded(action string, pkg *SheinPackage) bool {
	if pkg == nil || pkg.Submission == nil {
		return false
	}
	record := sheinSubmissionRecordForAction(pkg.Submission, action)
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
	var result *ListingKitResult
	if task != nil {
		result = task.Result
	}
	return s.persistSheinSubmitPhase(ctx, taskID, result, pkg, opts.action, opts.requestID, phase)
}

func (s *service) persistSheinSubmitPhase(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID, phase string) error {
	advanceSheinSubmitPhase(pkg, action, requestID, phase)
	appendSheinSubmissionEvent(pkg, buildSheinPhaseSubmissionEvent(taskID, action, phase, sheinpub.SubmissionStatusRunning, requestID, time.Now(), "", nil))
	if result == nil {
		return nil
	}
	result.UpdatedAt = time.Now()
	return s.repo.SaveTaskResult(ctx, taskID, result)
}

func (s *service) persistSuccessfulSheinDirectResponse(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions, supplierCode string, response *sheinpub.SubmissionResponse) error {
	if task == nil || task.Result == nil {
		return nil
	}
	setSheinSubmitRemoteResponse(pkg, opts.action, opts.requestID, supplierCode, response)
	task.Result.UpdatedAt = time.Now()
	if err := s.repo.SaveTaskResult(ctx, taskID, task.Result); err != nil {
		return err
	}
	return s.persistSheinDirectSubmitPhase(ctx, taskID, task, pkg, opts, sheinpub.SubmissionPhasePersistResult)
}

func (s *service) finishSheinDirectSubmitAttempt(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, opts sheinDirectSubmitOptions, response *sheinpub.SubmissionResponse, responseErr error) error {
	record := completeSheinSubmitAttempt(pkg, opts.action, opts.requestID, response, responseErr, time.Now())
	appendSheinSubmissionEvent(pkg, buildSheinSubmissionEvent(taskID, opts.action, record, response, responseErr, opts.startedAt))
	if responseErr == nil {
		s.rememberSheinSubmittedResolution(task, opts.action)
	}
	if err := s.persistSuccessfulSheinSubmission(ctx, taskID, task, opts.action); err != nil {
		return err
	}
	return responseErr
}

func (s *service) recordSheinSubmissionFailure(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action string, submitErr error) error {
	return s.recordSheinSubmissionFailureForState(ctx, taskID, result, pkg, action, "", "", submitErr)
}

func (s *service) recordSheinSubmissionFailureForState(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestedID, phase string, submitErr error) error {
	requestID := strings.TrimSpace(requestedID)
	phase = strings.TrimSpace(phase)
	if phase == "" {
		phase = sheinpub.SubmissionPhaseValidate
	}
	if pkg != nil && pkg.Submission != nil {
		if requestID == "" {
			requestID = pkg.Submission.CurrentRequestID
		}
		if phase == sheinpub.SubmissionPhaseValidate && pkg.Submission.CurrentPhase != "" {
			phase = pkg.Submission.CurrentPhase
		}
	}
	record := failSheinSubmitAttempt(pkg, action, requestID, phase, submitErr, time.Now())
	startedAt := record.SubmittedAt
	if !record.StartedAt.IsZero() {
		startedAt = record.StartedAt
	}
	appendSheinSubmissionEvent(pkg, buildSheinSubmissionEvent(taskID, action, record, nil, submitErr, startedAt))
	if result == nil {
		return nil
	}
	result.UpdatedAt = time.Now()
	return s.repo.SaveTaskResult(ctx, taskID, result)
}

func (s *service) failSheinDirectSubmit(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, action string, submitErr error) error {
	if task == nil {
		return submitErr
	}
	if saveErr := s.recordSheinSubmissionFailure(ctx, taskID, task.Result, pkg, action, submitErr); saveErr != nil {
		return saveErr
	}
	return submitErr
}

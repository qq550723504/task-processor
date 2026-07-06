package listingkit

import (
	"context"
	"time"

	apperrors "task-processor/internal/core/errors"
	submissiondomain "task-processor/internal/listing/submission"
	"task-processor/internal/listingkit/core"
	sheinpub "task-processor/internal/publishing/shein"
)

type sheinSubmissionRefreshMutationRequest struct {
	taskID       string
	action       string
	requestID    string
	startedAt    time.Time
	confirmation *sheinpub.SubmissionConfirmRemoteUpdate
	remoteErr    error
}

func (s *taskSubmissionRefreshService) persistSheinSubmissionRefreshResult(ctx context.Context, taskID string, refreshState *sheinSubmissionRefreshState, confirmation *sheinpub.SubmissionConfirmRemoteUpdate, remoteErr error) (*Task, error) {
	request, err := buildSubmissionRefreshMutationRequest(taskID, refreshState, confirmation, remoteErr)
	if err != nil {
		return nil, err
	}
	return s.mutateSubmissionRefreshTask(ctx, taskID, func(task *Task) error {
		return applySubmissionRefreshMutation(task, request)
	})
}

func buildSubmissionRefreshMutationRequest(taskID string, refreshState *sheinSubmissionRefreshState, confirmation *sheinpub.SubmissionConfirmRemoteUpdate, remoteErr error) (*sheinSubmissionRefreshMutationRequest, error) {
	remoteRequest, err := buildSheinRemoteStatusRequest(taskID, refreshState)
	if err != nil {
		return nil, err
	}
	return &sheinSubmissionRefreshMutationRequest{
		taskID:       taskID,
		action:       remoteRequest.action,
		requestID:    remoteRequest.requestID,
		startedAt:    remoteRequest.startedAt,
		confirmation: confirmation,
		remoteErr:    remoteErr,
	}, nil
}

func applySubmissionRefreshMutation(task *Task, request *sheinSubmissionRefreshMutationRequest) error {
	if request == nil {
		return apperrors.New(apperrors.ErrCodeSystem, "submission refresh mutation request is not available")
	}
	pkg, err := validateSubmissionRefreshMutation(task, request.action, request.requestID)
	if err != nil {
		return err
	}
	appendSubmissionRefreshMutationEvents(pkg, request)
	task.Result.UpdatedAt = time.Now()
	return nil
}

func appendSubmissionRefreshMutationEvents(pkg *SheinPackage, request *sheinSubmissionRefreshMutationRequest) {
	if pkg == nil || request == nil {
		return
	}
	sheinpub.AppendSubmissionEvent(pkg, sheinpub.BuildSubmissionRefreshConfirmRemoteRunningEvent(request.taskID, request.action, request.requestID, request.startedAt))
	applySubmissionRefreshConfirmation(pkg, request.action, request.requestID, request.confirmation)
	appendSubmissionRefreshFailureEvent(pkg, request)
}

func validateSubmissionRefreshMutation(task *Task, action, requestID string) (*SheinPackage, error) {
	pkg, err := loadSubmissionRefreshTaskPackage(task)
	if err != nil {
		return nil, err
	}
	selection := sheinpub.ResolveSubmissionRefreshSelection(pkg)
	if !submissiondomain.RefreshActionMatches(selection.Action, action) {
		return nil, buildSubmissionRefreshChangedError()
	}
	record := sheinpub.SubmissionRecordForAction(pkg.SubmissionState, action)
	if record == nil || !submissiondomain.RefreshRequestMatches(record.RequestID, requestID) {
		return nil, buildSubmissionRefreshChangedError()
	}
	return pkg, nil
}

func loadSubmissionRefreshTaskPackage(task *Task) (*SheinPackage, error) {
	if task == nil || task.Result == nil {
		return nil, buildSubmissionRefreshUnavailableError()
	}
	pkg, ok := sheinpub.SubmissionStatePackage(task.Result.Shein)
	if !ok {
		return nil, buildSubmissionRefreshUnavailableError()
	}
	return pkg, nil
}

func buildSubmissionRefreshUnavailableError() error {
	return apperrors.Wrap(ErrSubmitBlocked, apperrors.ErrCodeValidation, "shein submission is not available")
}

func buildSubmissionRefreshChangedError() error {
	return apperrors.Wrap(core.ErrSubmitInProgress, apperrors.ErrCodeTaskProcessing, "shein submission changed during refresh")
}

func applySubmissionRefreshConfirmation(pkg *SheinPackage, action, requestID string, confirmation *sheinpub.SubmissionConfirmRemoteUpdate) {
	if pkg == nil || confirmation == nil {
		return
	}
	sheinpub.ApplySubmissionConfirmRemoteUpdate(pkg, action, requestID, *confirmation)
}

func appendSubmissionRefreshFailureEvent(pkg *SheinPackage, request *sheinSubmissionRefreshMutationRequest) {
	if pkg == nil || request == nil || request.remoteErr == nil || request.confirmation != nil {
		return
	}
	event := sheinpub.BuildSubmissionConfirmRemoteEvent(
		request.taskID,
		request.action,
		sheinpub.SubmissionStatusFailed,
		request.requestID,
		request.startedAt,
		"刷新 SHEIN 远端提交状态失败",
		request.remoteErr,
	)
	sheinpub.AppendSubmissionEvent(pkg, event)
}

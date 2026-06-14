package listingkit

import (
	"context"
	"time"

	apperrors "task-processor/internal/core/errors"
	"task-processor/internal/listingkit/core"
	sheinpub "task-processor/internal/publishing/shein"
)

type sheinSubmissionRefreshMutationRequest struct {
	taskID       string
	action       string
	requestID    string
	startedAt    time.Time
	confirmation *sheinRemoteConfirmation
}

type sheinSubmissionRefreshValidationRequest struct {
	task      *Task
	action    string
	requestID string
}

func (s *taskSubmissionRefreshService) persistSheinSubmissionRefreshResult(ctx context.Context, taskID string, refreshState *sheinSubmissionRefreshState, confirmation *sheinRemoteConfirmation) (*Task, error) {
	request, err := buildSubmissionRefreshMutationRequest(taskID, refreshState, confirmation)
	if err != nil {
		return nil, err
	}
	return s.mutateSubmissionRefreshTask(ctx, taskID, func(task *Task) error {
		return applySubmissionRefreshMutation(task, request)
	})
}

func buildSubmissionRefreshMutationRequest(taskID string, refreshState *sheinSubmissionRefreshState, confirmation *sheinRemoteConfirmation) (*sheinSubmissionRefreshMutationRequest, error) {
	if refreshState == nil {
		return nil, apperrors.New(apperrors.ErrCodeSystem, "submission refresh state is not available")
	}
	return &sheinSubmissionRefreshMutationRequest{
		taskID:       taskID,
		action:       refreshState.action,
		requestID:    refreshState.requestID,
		startedAt:    refreshState.startedAt,
		confirmation: confirmation,
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
	appendSheinSubmissionEvent(pkg, sheinpub.BuildSubmissionRefreshConfirmRemoteRunningEvent(request.taskID, request.action, request.requestID, request.startedAt))
	applySubmissionRefreshConfirmation(pkg, request.action, request.requestID, request.confirmation)
}

func validateSubmissionRefreshMutation(task *Task, action, requestID string) (*SheinPackage, error) {
	request := buildSubmissionRefreshValidationRequest(task, action, requestID)
	pkg, err := loadSubmissionRefreshMutationPackage(request.task)
	if err != nil {
		return nil, err
	}
	if err := validateSubmissionRefreshAction(pkg, request.action); err != nil {
		return nil, err
	}
	if err := validateSubmissionRefreshRequest(pkg, request.action, request.requestID); err != nil {
		return nil, err
	}
	return pkg, nil
}

func buildSubmissionRefreshValidationRequest(task *Task, action, requestID string) *sheinSubmissionRefreshValidationRequest {
	return &sheinSubmissionRefreshValidationRequest{
		task:      task,
		action:    action,
		requestID: requestID,
	}
}

func loadSubmissionRefreshMutationPackage(task *Task) (*SheinPackage, error) {
	return loadSubmissionRefreshTaskPackage(task)
}

func loadSubmissionRefreshTaskPackage(task *Task) (*SheinPackage, error) {
	if task == nil || task.Result == nil {
		return nil, buildSubmissionRefreshUnavailableError()
	}
	pkg, err := loadSubmissionRefreshPackageState(task.Result.Shein)
	if err != nil {
		return nil, err
	}
	return pkg, nil
}

func validateSubmissionRefreshAction(pkg *SheinPackage, action string) error {
	if _, err := loadSubmissionRefreshPackageState(pkg); err != nil {
		return err
	}
	if !sheinpub.SubmissionRefreshActionMatches(pkg, action) {
		return buildSubmissionRefreshChangedError()
	}
	return nil
}

func validateSubmissionRefreshRequest(pkg *SheinPackage, action, requestID string) error {
	if _, err := loadSubmissionRefreshPackageState(pkg); err != nil {
		return err
	}
	if !sheinpub.SubmissionRefreshRequestMatches(pkg, action, requestID) {
		return buildSubmissionRefreshChangedError()
	}
	return nil
}

func loadSubmissionRefreshPackageState(pkg *SheinPackage) (*SheinPackage, error) {
	var ok bool
	pkg, ok = sheinpub.SubmissionStatePackage(pkg)
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

func applySubmissionRefreshConfirmation(pkg *SheinPackage, action, requestID string, confirmation *sheinRemoteConfirmation) {
	if pkg == nil || confirmation == nil {
		return
	}
	if sheinpub.ApplySubmissionConfirmRemoteWithEvent(pkg, action, requestID, confirmation.remoteStatus, confirmation.record, confirmation.checkedAt, confirmation.message, confirmation.event) {
		return
	}
	applySubmissionRefreshRemoteRecord(pkg, action, requestID, confirmation)
}

func applySubmissionRefreshRemoteRecord(pkg *SheinPackage, action, requestID string, confirmation *sheinRemoteConfirmation) {
	if pkg == nil || confirmation == nil {
		return
	}
	setSheinSubmitRemoteRecord(pkg, action, requestID, confirmation.remoteStatus, confirmation.record, confirmation.checkedAt, confirmation.message)
}

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
}

func validateSubmissionRefreshMutation(task *Task, action, requestID string) (*SheinPackage, error) {
	request := buildSubmissionRefreshValidationRequest(task, action, requestID)
	pkg, err := loadSubmissionRefreshMutationPackage(request.task)
	if err != nil {
		return nil, err
	}
	validation := sheinpub.ResolveSubmissionRefreshValidation(pkg, request.action, request.requestID)
	if !validation.Available {
		return nil, buildSubmissionRefreshUnavailableError()
	}
	if !validation.ActionMatches {
		return nil, buildSubmissionRefreshChangedError()
	}
	if !validation.RequestMatches {
		return nil, buildSubmissionRefreshChangedError()
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
	validation := sheinpub.ResolveSubmissionRefreshValidation(pkg, action, "")
	if !validation.Available {
		return buildSubmissionRefreshUnavailableError()
	}
	if !validation.ActionMatches {
		return buildSubmissionRefreshChangedError()
	}
	return nil
}

func validateSubmissionRefreshRequest(pkg *SheinPackage, action, requestID string) error {
	validation := sheinpub.ResolveSubmissionRefreshValidation(pkg, action, requestID)
	if !validation.Available {
		return buildSubmissionRefreshUnavailableError()
	}
	if !validation.RequestMatches {
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
	sheinpub.ApplySubmissionConfirmRemoteUpdate(pkg, action, requestID, *confirmation)
}

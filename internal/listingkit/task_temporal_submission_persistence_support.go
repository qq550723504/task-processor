package listingkit

import (
	"context"
	"errors"
	"strings"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	"task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

func applySheinTemporalSubmissionPersistenceInput(pkg *SheinPackage, action, requestID, supplierCode string, response *sheinpub.SubmissionResponse, snapshot *sheinpub.SubmitSnapshot) {
	if snapshot != nil {
		setSheinSubmitSnapshot(pkg, action, requestID, snapshot)
	}
	if supplierCode != "" {
		setSheinSubmitSupplierCode(pkg, action, requestID, supplierCode)
	}
	if response != nil {
		setSheinSubmitRemoteResponse(pkg, action, requestID, supplierCode, response)
	}
}

func (s *taskTemporalSubmissionAdapter) persistSheinTemporalSubmissionSuccess(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, action, requestID string, response *sheinpub.SubmissionResponse) error {
	startedAt := sheinSubmitStartedAt(pkg, action, requestID, time.Now())
	if s.successRunner != nil {
		return s.successRunner.PersistSuccess(ctx, submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse]{
			TaskID:    taskID,
			Task:      task,
			Package:   pkg,
			Action:    action,
			RequestID: requestID,
			Response:  response,
			StartedAt: startedAt,
		})
	}
	task.Result.UpdatedAt = time.Now()
	if err := s.saveTaskResult(ctx, taskID, task.Result); err != nil {
		return err
	}
	if err := s.persistSheinSubmitPhase(ctx, taskID, task.Result, pkg, action, requestID, sheinpub.SubmissionPhasePersistResult); err != nil {
		return err
	}
	record := completeSheinSubmitAttempt(pkg, action, requestID, response, nil, time.Now())
	appendSheinSubmissionEvent(pkg, submission.BuildEvent(taskID, action, record, response, nil, startedAt))
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, action)
	}
	return s.persistSuccessfulSheinSubmission(ctx, taskID, task, action)
}

func (s *taskTemporalSubmissionAdapter) persistTemporalSuccessResultAndPhase(ctx context.Context, in submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse]) error {
	in.Task.Result.UpdatedAt = time.Now()
	if err := s.saveTaskResult(ctx, in.TaskID, in.Task.Result); err != nil {
		return err
	}
	return s.persistSheinSubmitPhase(ctx, in.TaskID, in.Task.Result, in.Package, in.Action, in.RequestID, sheinpub.SubmissionPhasePersistResult)
}

func (s *taskTemporalSubmissionAdapter) completeTemporalSubmitAttempt(in submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse], finishedAt time.Time) {
	record := completeSheinSubmitAttempt(in.Package, in.Action, in.RequestID, in.Response, nil, finishedAt)
	appendSheinSubmissionEvent(in.Package, submission.BuildEvent(in.TaskID, in.Action, record, in.Response, nil, in.StartedAt))
}

func (s *taskTemporalSubmissionAdapter) persistSheinTemporalSubmissionFailure(ctx context.Context, taskID string, result *ListingKitResult, pkg *SheinPackage, action, requestID, phase, errorMessage string) error {
	input := submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]{
		TaskID:    taskID,
		Result:    result,
		Package:   pkg,
		Action:    action,
		RequestID: requestID,
		Phase:     phase,
		Err:       errors.New(strings.TrimSpace(errorMessage)),
	}
	if s.failureRunner != nil {
		return s.failureRunner.PersistFailure(ctx, input)
	}
	return s.recordTemporalFailureState(ctx, input)
}

func (s *taskTemporalSubmissionAdapter) finishSheinTemporalRemoteRefreshFailure(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, action, requestID string, response *sheinpub.SubmissionResponse, remoteErr error) error {
	record := failSheinSubmitAttempt(pkg, action, requestID, sheinpub.SubmissionPhaseConfirmRemote, remoteErr, time.Now())
	appendSheinSubmissionEvent(pkg, submission.BuildEvent(taskID, action, record, response, remoteErr, record.StartedAt))
	task.Result.UpdatedAt = time.Now()
	return s.saveTaskResult(ctx, taskID, task.Result)
}

func (s *taskTemporalSubmissionAdapter) finishSheinTemporalRemoteRefreshSuccess(ctx context.Context, taskID string, task *Task, pkg *SheinPackage, action, requestID string, response *sheinpub.SubmissionResponse) (*SheinRefreshRemoteStatusResult, error) {
	response = confirmedSubmissionResponse(response, action)
	record := completeSheinSubmitAttempt(pkg, action, requestID, response, nil, time.Now())
	appendSheinSubmissionEvent(pkg, submission.BuildEvent(taskID, action, record, record.Result, nil, record.StartedAt))
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, action)
	}
	if err := s.persistSuccessfulSheinSubmission(ctx, taskID, task, action); err != nil {
		return nil, err
	}

	remoteStatus := ""
	if pkg.SubmissionState != nil {
		remoteStatus = pkg.SubmissionState.RemoteStatus
	}
	return &SheinRefreshRemoteStatusResult{
		TaskID:       taskID,
		Action:       action,
		RequestID:    requestID,
		RemoteStatus: remoteStatus,
	}, nil
}

func (s *taskTemporalSubmissionAdapter) recordTemporalFailureState(ctx context.Context, in submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]) error {
	return s.recordSheinSubmissionFailureForState(ctx, in.TaskID, in.Result, in.Package, in.Action, in.RequestID, in.Phase, in.Err)
}

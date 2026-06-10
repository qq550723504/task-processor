package listingkit

import (
	"context"
	"errors"
	"strings"
	"time"

	"task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

func (s *taskTemporalSubmissionAdapter) PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error {
	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return err
	}
	if in.Snapshot != nil {
		setSheinSubmitSnapshot(pkg, in.Action, in.RequestID, in.Snapshot)
	}
	setSheinSubmitRemoteResponse(pkg, in.Action, in.RequestID, in.SupplierCode, in.Response)
	task.Result.UpdatedAt = time.Now()
	if err := s.saveTaskResult(ctx, in.TaskID, task.Result); err != nil {
		return err
	}
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhasePersistResult); err != nil {
		return err
	}

	startedAt := sheinSubmitStartedAt(pkg, in.Action, in.RequestID, time.Now())
	record := completeSheinSubmitAttempt(pkg, in.Action, in.RequestID, in.Response, nil, time.Now())
	appendSheinSubmissionEvent(pkg, submission.BuildEvent(in.TaskID, in.Action, record, in.Response, nil, startedAt))
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, in.Action)
	}
	return s.persistSuccessfulSheinSubmission(ctx, in.TaskID, task, in.Action)
}

func (s *taskTemporalSubmissionAdapter) PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error {
	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return err
	}
	if in.Snapshot != nil {
		setSheinSubmitSnapshot(pkg, in.Action, in.RequestID, in.Snapshot)
	}
	if in.SupplierCode != "" {
		setSheinSubmitSupplierCode(pkg, in.Action, in.RequestID, in.SupplierCode)
	}
	if in.Response != nil {
		setSheinSubmitRemoteResponse(pkg, in.Action, in.RequestID, in.SupplierCode, in.Response)
	}
	return s.recordSheinSubmissionFailureForState(
		ctx,
		in.TaskID,
		task.Result,
		pkg,
		in.Action,
		in.RequestID,
		in.Phase,
		errors.New(strings.TrimSpace(in.ErrorMessage)),
	)
}

func (s *taskTemporalSubmissionAdapter) RefreshSheinPublishRemoteStatus(ctx context.Context, in SheinRefreshRemoteStatusInput) (*SheinRefreshRemoteStatusResult, error) {
	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, err
	}
	if err := s.persistSheinSubmitPhase(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, sheinpub.SubmissionPhaseConfirmRemote); err != nil {
		return nil, err
	}

	startedAt := sheinSubmitStartedAt(pkg, in.Action, in.RequestID, time.Now())
	remoteEvent, remoteErr := s.refreshSheinSubmitRemoteStatus(ctx, task, in.TaskID, pkg, productAPI, in.Action, in.RequestID, in.SupplierCode, startedAt)
	if remoteEvent != nil {
		appendSheinSubmissionEvent(pkg, *remoteEvent)
	}

	record := sheinSubmissionRecordForAction(pkg.SubmissionState, in.Action)
	response := submissionResponseForRecord(pkg, in.Action)
	if remoteErr != nil {
		record = failSheinSubmitAttempt(pkg, in.Action, in.RequestID, sheinpub.SubmissionPhaseConfirmRemote, remoteErr, time.Now())
		appendSheinSubmissionEvent(pkg, submission.BuildEvent(in.TaskID, in.Action, record, response, remoteErr, record.StartedAt))
		task.Result.UpdatedAt = time.Now()
		if err := s.saveTaskResult(ctx, in.TaskID, task.Result); err != nil {
			return nil, err
		}
		return nil, remoteErr
	}

	response = confirmedSubmissionResponse(response, in.Action)
	record = completeSheinSubmitAttempt(pkg, in.Action, in.RequestID, response, nil, time.Now())
	appendSheinSubmissionEvent(pkg, submission.BuildEvent(in.TaskID, in.Action, record, record.Result, nil, record.StartedAt))
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, in.Action)
	}
	if err := s.persistSuccessfulSheinSubmission(ctx, in.TaskID, task, in.Action); err != nil {
		return nil, err
	}

	remoteStatus := ""
	if pkg.SubmissionState != nil {
		remoteStatus = pkg.SubmissionState.RemoteStatus
	}
	return &SheinRefreshRemoteStatusResult{
		TaskID:       in.TaskID,
		Action:       in.Action,
		RequestID:    in.RequestID,
		RemoteStatus: remoteStatus,
	}, nil
}

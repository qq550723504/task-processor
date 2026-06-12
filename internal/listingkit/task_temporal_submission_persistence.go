package listingkit

import (
	"context"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

func (s *taskTemporalSubmissionAdapter) PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error {
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return err
	}
	applySheinTemporalSubmissionPersistenceInput(pkg, in.Action, in.RequestID, in.SupplierCode, in.Response, in.Snapshot)
	return s.persistSheinTemporalSubmissionSuccess(ctx, in.TaskID, task, pkg, in.Action, in.RequestID, in.Response)
}

func (s *taskTemporalSubmissionAdapter) PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error {
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
	if err != nil {
		return err
	}
	applySheinTemporalSubmissionPersistenceInput(pkg, in.Action, in.RequestID, in.SupplierCode, in.Response, in.Snapshot)
	return s.persistSheinTemporalSubmissionFailure(ctx, in.TaskID, task.Result, pkg, in.Action, in.RequestID, in.Phase, in.ErrorMessage)
}

func (s *taskTemporalSubmissionAdapter) RefreshSheinPublishRemoteStatus(ctx context.Context, in SheinRefreshRemoteStatusInput) (*SheinRefreshRemoteStatusResult, error) {
	task, pkg, err := s.loadSheinPublishTaskState(ctx, in.TaskID)
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

	response := submissionResponseForRecord(pkg, in.Action)
	if remoteErr != nil {
		if err := s.finishSheinTemporalRemoteRefreshFailure(ctx, in.TaskID, task, pkg, in.Action, in.RequestID, response, remoteErr); err != nil {
			return nil, err
		}
		return nil, remoteErr
	}

	return s.finishSheinTemporalRemoteRefreshSuccess(ctx, in.TaskID, task, pkg, in.Action, in.RequestID, response)
}

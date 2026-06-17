package listingkit

import (
	"context"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type taskTemporalSubmissionRefreshServiceConfig struct {
	loadSheinPublishTask           func(context.Context, string) (*Task, *SheinPackage, error)
	buildSheinSubmitProductAPI     func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	persistSheinSubmitPhase        func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string) error
	refreshSheinSubmitRemoteStatus func(context.Context, *sheinRemoteRefreshRequest) (*sheinpub.SubmissionEvent, error)
	persistence                    *taskTemporalSubmissionPersistenceService
}

type sheinTemporalRemoteRefreshState = sheinRemoteRefreshExecutionState

type taskTemporalSubmissionRefreshService struct {
	loadSheinPublishTask           func(context.Context, string) (*Task, *SheinPackage, error)
	buildSheinSubmitProductAPI     func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	persistSheinSubmitPhase        func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string) error
	refreshSheinSubmitRemoteStatus func(context.Context, *sheinRemoteRefreshRequest) (*sheinpub.SubmissionEvent, error)
	persistence                    *taskTemporalSubmissionPersistenceService
	remoteRefreshRunner            *submissiondomain.RemoteRefreshService[sheinTemporalRemoteRefreshState, *sheinRemoteRefreshRequest, *sheinpub.SubmissionEvent, SheinRefreshRemoteStatusResult]
}

func newTaskTemporalSubmissionRefreshService(config taskTemporalSubmissionRefreshServiceConfig) *taskTemporalSubmissionRefreshService {
	service := &taskTemporalSubmissionRefreshService{
		loadSheinPublishTask:           config.loadSheinPublishTask,
		buildSheinSubmitProductAPI:     config.buildSheinSubmitProductAPI,
		persistSheinSubmitPhase:        config.persistSheinSubmitPhase,
		refreshSheinSubmitRemoteStatus: config.refreshSheinSubmitRemoteStatus,
		persistence:                    config.persistence,
	}
	service.remoteRefreshRunner = submissiondomain.NewRemoteRefreshService(submissiondomain.RemoteRefreshServiceConfig[sheinTemporalRemoteRefreshState, *sheinRemoteRefreshRequest, *sheinpub.SubmissionEvent, SheinRefreshRemoteStatusResult]{
		PersistPhase: service.persistTemporalRemoteRefreshPhase,
		BuildRequest: service.buildTemporalRemoteRefreshRequest,
		Execute:      service.refreshSheinSubmitRemoteStatus,
		RecordEvent:  service.recordTemporalRemoteRefreshEvent,
		FinishError:  service.finishTemporalRemoteRefreshError,
		FinishOK:     service.finishTemporalRemoteRefreshSuccess,
	})
	return service
}

func (s *taskTemporalSubmissionRefreshService) RefreshSheinPublishRemoteStatus(ctx context.Context, in SheinRefreshRemoteStatusInput) (*SheinRefreshRemoteStatusResult, error) {
	if s.loadSheinPublishTask == nil {
		return nil, ErrTaskResultUnavailable
	}
	task, pkg, err := s.loadSheinPublishTask(ctx, in.TaskID)
	if err != nil {
		return nil, err
	}
	refreshState := buildSheinTemporalRemoteRefreshState(task, pkg, in, time.Now())
	return s.remoteRefreshRunner.Refresh(ctx, refreshState)
}

func buildSheinTemporalRemoteRefreshState(task *Task, pkg *SheinPackage, in SheinRefreshRemoteStatusInput, fallbackStartedAt time.Time) *sheinTemporalRemoteRefreshState {
	startedAt := sheinpub.SubmissionStartedAt(pkg, in.Action, in.RequestID, fallbackStartedAt)
	return newSheinRemoteRefreshExecutionState(sheinRemoteCompletionState{
		taskID:    in.TaskID,
		task:      task,
		pkg:       pkg,
		action:    in.Action,
		requestID: in.RequestID,
		startedAt: startedAt,
		response:  sheinpub.SubmissionResponseForAction(pkg, in.Action),
	}, in.SupplierCode, startedAt)
}

func (s *taskTemporalSubmissionRefreshService) persistTemporalRemoteRefreshPhase(ctx context.Context, state *sheinTemporalRemoteRefreshState) error {
	if state == nil || state.completion.task == nil || state.completion.pkg == nil {
		return ErrTaskResultUnavailable
	}
	return s.persistSheinSubmitPhase(ctx, state.completion.taskID, state.completion.task.Result, state.completion.pkg, state.completion.action, state.completion.requestID, sheinpub.SubmissionPhaseConfirmRemote)
}

func (s *taskTemporalSubmissionRefreshService) buildTemporalRemoteRefreshRequest(ctx context.Context, state *sheinTemporalRemoteRefreshState) (*sheinRemoteRefreshRequest, error) {
	if state == nil || state.completion.task == nil {
		return nil, ErrTaskResultUnavailable
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, state.completion.task)
	if err != nil {
		return nil, err
	}
	return buildSheinRemoteRefreshRequest(productAPI, state), nil
}

func (s *taskTemporalSubmissionRefreshService) recordTemporalRemoteRefreshEvent(state *sheinTemporalRemoteRefreshState, event *sheinpub.SubmissionEvent) {
	if state == nil || state.completion.pkg == nil || event == nil {
		return
	}
	sheinpub.AppendSubmissionEvent(state.completion.pkg, *event)
}

func (s *taskTemporalSubmissionRefreshService) finishTemporalRemoteRefreshError(ctx context.Context, state *sheinTemporalRemoteRefreshState, remoteErr error) (*SheinRefreshRemoteStatusResult, error) {
	if s.persistence == nil {
		return nil, nil
	}
	return nil, s.persistence.finishSheinTemporalRemoteRefreshFailure(ctx, state, remoteErr)
}

func (s *taskTemporalSubmissionRefreshService) finishTemporalRemoteRefreshSuccess(ctx context.Context, state *sheinTemporalRemoteRefreshState) (*SheinRefreshRemoteStatusResult, error) {
	if s.persistence == nil {
		return nil, nil
	}
	return s.persistence.finishSheinTemporalRemoteRefreshSuccess(ctx, state)
}

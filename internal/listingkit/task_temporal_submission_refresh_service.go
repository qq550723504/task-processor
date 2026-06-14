package listingkit

import (
	"context"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type taskTemporalSubmissionRefreshServiceConfig struct {
	loadSheinPublishTask           func(context.Context, string) (*Task, *SheinPackage, error)
	buildSheinSubmitProductAPI     func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	persistSheinSubmitPhase        func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string) error
	refreshSheinSubmitRemoteStatus func(context.Context, *Task, string, *SheinPackage, sheinproduct.ProductAPI, string, string, string, time.Time) (*sheinpub.SubmissionEvent, error)
	persistence                    *taskTemporalSubmissionPersistenceService
}

type taskTemporalSubmissionRefreshService struct {
	loadSheinPublishTask           func(context.Context, string) (*Task, *SheinPackage, error)
	buildSheinSubmitProductAPI     func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	persistSheinSubmitPhase        func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string) error
	refreshSheinSubmitRemoteStatus func(context.Context, *Task, string, *SheinPackage, sheinproduct.ProductAPI, string, string, string, time.Time) (*sheinpub.SubmissionEvent, error)
	persistence                    *taskTemporalSubmissionPersistenceService
}

func newTaskTemporalSubmissionRefreshService(config taskTemporalSubmissionRefreshServiceConfig) *taskTemporalSubmissionRefreshService {
	return &taskTemporalSubmissionRefreshService{
		loadSheinPublishTask:           config.loadSheinPublishTask,
		buildSheinSubmitProductAPI:     config.buildSheinSubmitProductAPI,
		persistSheinSubmitPhase:        config.persistSheinSubmitPhase,
		refreshSheinSubmitRemoteStatus: config.refreshSheinSubmitRemoteStatus,
		persistence:                    config.persistence,
	}
}

func (s *taskTemporalSubmissionRefreshService) RefreshSheinPublishRemoteStatus(ctx context.Context, in SheinRefreshRemoteStatusInput) (*SheinRefreshRemoteStatusResult, error) {
	if s.loadSheinPublishTask == nil {
		return nil, ErrTaskResultUnavailable
	}
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

	selection := sheinpub.ResolveSubmissionRemoteRefreshSelection(pkg, in.Action, in.RequestID, time.Now())
	startedAt := selection.StartedAt
	remoteEvent, remoteErr := s.refreshSheinSubmitRemoteStatus(ctx, task, in.TaskID, pkg, productAPI, in.Action, in.RequestID, in.SupplierCode, startedAt)
	if remoteEvent != nil {
		appendSheinSubmissionEvent(pkg, *remoteEvent)
	}

	response := selection.Response
	if remoteErr != nil {
		if s.persistence == nil {
			return nil, remoteErr
		}
		if err := s.persistence.finishSheinTemporalRemoteRefreshFailure(ctx, in.TaskID, task, pkg, in.Action, in.RequestID, response, remoteErr); err != nil {
			return nil, err
		}
		return nil, remoteErr
	}

	if s.persistence == nil {
		return nil, nil
	}
	return s.persistence.finishSheinTemporalRemoteRefreshSuccess(ctx, in.TaskID, task, pkg, in.Action, in.RequestID, response)
}

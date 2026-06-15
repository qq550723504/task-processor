package listingkit

import (
	"context"
	"errors"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

type taskSubmissionRecoveryServiceConfig struct {
	repo                        Repository
	buildTaskPreview            func(context.Context, *Task, string) (*ListingKitPreview, error)
	buildSheinSubmitProductAPI  func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	buildSheinSubmitOtherAPI    func(context.Context, *Task) (sheinother.OtherAPI, error)
	rememberSheinSubmitted      func(*Task, string)
	persistSuccessfulSubmission func(context.Context, string, *Task, string) error
	recordSubmissionFailure     func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string, error) error
	resolveRemoteStatusCallback func(*sheinRemoteStatusRequest) (*sheinRemoteConfirmation, error)
}

type taskSubmissionRecoveryService struct {
	repo                        Repository
	buildTaskPreview            func(context.Context, *Task, string) (*ListingKitPreview, error)
	buildSheinSubmitProductAPI  func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	buildSheinSubmitOtherAPI    func(context.Context, *Task) (sheinother.OtherAPI, error)
	rememberSheinSubmitted      func(*Task, string)
	persistSuccessfulSubmission func(context.Context, string, *Task, string) error
	recordSubmissionFailure     func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string, error) error
	resolveRemoteStatusCallback func(*sheinRemoteStatusRequest) (*sheinRemoteConfirmation, error)
	leaseAcquireRunner          *submissiondomain.LeaseAcquireService[Task, ListingKitPreview]
	startFailureRunner          *submissiondomain.StartFailureService[sheinWorkflowStartFailureInput]
	recoveryRouteRunner         *submissiondomain.RecoveryRouteService[sheinRecoveredRemoteState, ListingKitPreview]
	remoteRefreshRunner         *submissiondomain.RemoteRefreshService[sheinRecoveredRemoteState, *sheinRemoteRefreshRequest, *sheinpub.SubmissionEvent, ListingKitPreview]
}

type sheinRecoveredRemoteState struct {
	completion sheinRemoteCompletionState
	selection  sheinpub.SubmissionRecoverySelection
	now        time.Time
}

type sheinWorkflowStartFailureInput struct {
	taskID    string
	task      *Task
	action    string
	requestID string
	startErr  error
}

func newTaskSubmissionRecoveryService(config taskSubmissionRecoveryServiceConfig) *taskSubmissionRecoveryService {
	service := &taskSubmissionRecoveryService{
		repo:                        config.repo,
		buildTaskPreview:            config.buildTaskPreview,
		buildSheinSubmitProductAPI:  config.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:    config.buildSheinSubmitOtherAPI,
		rememberSheinSubmitted:      config.rememberSheinSubmitted,
		persistSuccessfulSubmission: config.persistSuccessfulSubmission,
		recordSubmissionFailure:     config.recordSubmissionFailure,
		resolveRemoteStatusCallback: config.resolveRemoteStatusCallback,
	}
	service.remoteRefreshRunner = submissiondomain.NewRemoteRefreshService(submissiondomain.RemoteRefreshServiceConfig[sheinRecoveredRemoteState, *sheinRemoteRefreshRequest, *sheinpub.SubmissionEvent, ListingKitPreview]{
		PersistPhase: service.persistRecoveredRemoteRefreshPhase,
		BuildRequest: service.buildRecoveredRemoteRefreshRequest,
		Execute:      service.refreshSheinSubmitRemoteStatus,
		RecordEvent:  service.recordRecoveredRemoteRefreshEvent,
		FinishError:  service.finishRecoveredRemoteRefreshError,
		FinishOK:     service.finishRecoveredRemoteRefreshSuccess,
	})
	service.recoveryRouteRunner = submissiondomain.NewRecoveryRouteService(submissiondomain.RecoveryRouteServiceConfig[sheinRecoveredRemoteState, ListingKitPreview]{
		UseLocal:      service.shouldRecoverLocally,
		RecoverLocal:  service.recoverSheinSubmitLocally,
		RecoverRemote: service.recoverSheinSubmitViaRemoteConfirmation,
	})
	service.leaseAcquireRunner = submissiondomain.NewLeaseAcquireService(submissiondomain.LeaseAcquireServiceConfig[Task, ListingKitPreview]{
		BeginLease:         service.beginSheinSubmitLeaseWithoutStartTime,
		IsReplayExisting:   func(err error) bool { return errors.Is(err, errSheinSubmitReplayExisting) },
		IsRecoverRemote:    func(err error) bool { return errors.Is(err, errSheinSubmitRecoverRemote) },
		IsMissingPackage:   func(err error) bool { return errors.Is(err, errSheinSubmitMissingPackage) },
		BuildReplayPreview: service.buildSheinReplayPreview,
		RecoverRemote:      service.recoverSheinSubmitRemote,
		BuildMissingPkgErr: service.buildMissingPackageAcquireError,
	})
	service.startFailureRunner = submissiondomain.NewStartFailureService(submissiondomain.StartFailureServiceConfig[sheinWorkflowStartFailureInput]{
		RecordFailure: service.recordWorkflowStartFailure,
		ClearFailure:  service.clearWorkflowStartFailure,
		OriginalError: func(in sheinWorkflowStartFailureInput) error { return in.startErr },
	})
	return service
}

func (s *taskSubmissionRecoveryService) acquireSheinSubmitTask(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, *ListingKitPreview, error) {
	return s.leaseAcquireRunner.Acquire(context.WithValue(ctx, sheinSubmitStartedAtContextKey{}, startedAt), taskID, action, requestID)
}

func (s *taskSubmissionRecoveryService) mutateTaskResult(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error) {
	if txRepo, ok := s.repo.(TaskResultTransactionRepository); ok {
		return txRepo.MutateTaskResult(ctx, taskID, mutate)
	}
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if mutate != nil {
		if err := mutate(task); err != nil {
			return task, err
		}
	}
	if task.Result != nil {
		if err := s.repo.SaveTaskResult(ctx, taskID, task.Result); err != nil {
			return nil, err
		}
	}
	return task, nil
}

func (s *taskSubmissionRecoveryService) handleSheinWorkflowStartFailure(ctx context.Context, taskID string, task *Task, opts sheinWorkflowSubmitOptions, startErr error) error {
	return s.startFailureRunner.Handle(ctx, sheinWorkflowStartFailureInput{
		taskID:    taskID,
		task:      task,
		action:    opts.action,
		requestID: opts.requestID,
		startErr:  startErr,
	})
}

func (s *taskSubmissionRecoveryService) recoverSheinSubmitRemote(ctx context.Context, task *Task, action string) (*ListingKitPreview, error) {
	recoveredState, err := buildRecoveredSheinRemoteState(task, action)
	if err != nil {
		return nil, err
	}
	return s.executeRecoveredSheinSubmitRoute(ctx, recoveredState)
}

func (s *taskSubmissionRecoveryService) executeRecoveredSheinSubmitRoute(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	return s.recoveryRouteRunner.Recover(ctx, state)
}

type sheinSubmitStartedAtContextKey struct{}

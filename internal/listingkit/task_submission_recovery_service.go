package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	submissiondomain "task-processor/internal/listing/submission"
	"task-processor/internal/listingkit/core"
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

func (s *taskSubmissionRecoveryService) recoverSheinSubmitLocally(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if state == nil || state.completion.task == nil || state.completion.pkg == nil {
		return nil, ErrTaskResultUnavailable
	}
	sheinpub.AppendSubmissionEvent(state.completion.pkg, sheinpub.BuildSubmissionPhaseEvent(state.completion.taskID, state.completion.action, sheinpub.SubmissionPhasePersistResult, sheinpub.SubmissionStatusRunning, state.completion.requestID, state.now, "恢复本地提交完成状态", nil))
	if state.selection.Record != nil {
		if _, err := persistSheinRemoteCompletionSuccess(ctx, &state.completion, state.completion.response, s.rememberSheinSubmitted, s.persistSuccessfulSubmission); err != nil {
			return nil, err
		}
		return s.buildTaskPreview(ctx, state.completion.task, "shein")
	}
	return s.finalizeRecoveredSheinSubmission(ctx, state.completion.task, state.completion.action)
}

func loadRecoveredSheinSubmissionReport(task *Task) (*SheinPackage, error) {
	if task == nil || task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	pkg, ok := sheinpub.SubmissionStatePackage(task.Result.Shein)
	if !ok {
		return nil, ErrTaskResultUnavailable
	}
	return pkg, nil
}

func buildRecoveredSheinRemoteState(task *Task, action string) (*sheinRecoveredRemoteState, error) {
	pkg, err := loadRecoveredSheinSubmissionReport(task)
	if err != nil {
		return nil, err
	}
	selection := sheinpub.ResolveSubmissionRecoverySelection(pkg, action)
	if selection.Report == nil {
		return nil, ErrTaskResultUnavailable
	}
	taskID := ""
	if task != nil {
		taskID = task.ID
	}
	return &sheinRecoveredRemoteState{
		completion: sheinRemoteCompletionState{
			taskID:    taskID,
			task:      task,
			pkg:       pkg,
			action:    action,
			requestID: selection.RequestID,
			startedAt: selection.StartedAt,
			response:  selection.Response,
		},
		selection: selection,
		now:       time.Now(),
	}, nil
}

func buildRecoveredSheinRemoteRefreshState(state *sheinRecoveredRemoteState) *sheinRemoteRefreshExecutionState {
	if state == nil {
		return nil
	}
	return newSheinRemoteRefreshExecutionState(state.completion, state.selection.SupplierCode, state.now)
}

func (s *taskSubmissionRecoveryService) recoverSheinSubmitViaRemoteConfirmation(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if state == nil || state.completion.task == nil || state.completion.pkg == nil {
		return nil, ErrTaskResultUnavailable
	}
	if state.selection.Record == nil || strings.TrimSpace(state.selection.SupplierCode) == "" {
		return nil, fmt.Errorf("%w: stale SHEIN submit has no supplier code", core.ErrSubmitInProgress)
	}
	return s.remoteRefreshRunner.Refresh(ctx, state)
}

func (s *taskSubmissionRecoveryService) persistSheinRecoveredRemoteFailure(ctx context.Context, state *sheinRecoveredRemoteState, remoteErr error) error {
	if state == nil {
		return remoteErr
	}
	return persistSheinRemoteCompletionFailure(ctx, s.repo.SaveTaskResult, &state.completion, sheinpub.SubmissionPhaseConfirmRemote, remoteErr)
}

func (s *taskSubmissionRecoveryService) completeSheinRecoveredRemoteSuccess(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if state == nil || state.completion.task == nil || state.completion.pkg == nil {
		return nil, ErrTaskResultUnavailable
	}
	record, err := persistSheinRemoteCompletionSuccess(ctx, &state.completion, state.completion.response, s.rememberSheinSubmitted, s.persistSuccessfulSubmission)
	if err != nil {
		return nil, err
	}
	if state.selection.Record != nil && record != nil && record.Result == nil {
		record.Status = sheinpub.SubmissionStatusSuccess
	}
	return s.buildTaskPreview(ctx, state.completion.task, "shein")
}

func (s *taskSubmissionRecoveryService) finalizeRecoveredSheinSubmission(ctx context.Context, task *Task, action string) (*ListingKitPreview, error) {
	if task == nil {
		return nil, ErrTaskResultUnavailable
	}
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, action)
	}
	if s.persistSuccessfulSubmission != nil {
		if err := s.persistSuccessfulSubmission(ctx, task.ID, task, action); err != nil {
			return nil, err
		}
	}
	return s.buildTaskPreview(ctx, task, "shein")
}

func (s *taskSubmissionRecoveryService) persistRecoveredRemoteRefreshPhase(_ context.Context, state *sheinRecoveredRemoteState) error {
	if state == nil || state.completion.pkg == nil {
		return ErrTaskResultUnavailable
	}
	sheinpub.AppendSubmissionEvent(state.completion.pkg, advanceSheinSubmitPhaseAndBuildEvent(state.completion.pkg, state.completion.taskID, state.completion.action, state.completion.requestID, sheinpub.SubmissionPhaseConfirmRemote, state.now))
	return nil
}

func (s *taskSubmissionRecoveryService) buildRecoveredRemoteRefreshRequest(ctx context.Context, state *sheinRecoveredRemoteState) (*sheinRemoteRefreshRequest, error) {
	if state == nil || state.completion.task == nil {
		return nil, ErrTaskResultUnavailable
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, state.completion.task)
	if err != nil {
		return nil, err
	}
	return buildSheinRemoteRefreshRequest(productAPI, buildRecoveredSheinRemoteRefreshState(state)), nil
}

func (s *taskSubmissionRecoveryService) recordRecoveredRemoteRefreshEvent(state *sheinRecoveredRemoteState, event *sheinpub.SubmissionEvent) {
	if state == nil || state.completion.pkg == nil || event == nil {
		return
	}
	sheinpub.AppendSubmissionEvent(state.completion.pkg, *event)
}

func (s *taskSubmissionRecoveryService) finishRecoveredRemoteRefreshError(ctx context.Context, state *sheinRecoveredRemoteState, remoteErr error) (*ListingKitPreview, error) {
	return nil, s.persistSheinRecoveredRemoteFailure(ctx, state, remoteErr)
}

func (s *taskSubmissionRecoveryService) finishRecoveredRemoteRefreshSuccess(ctx context.Context, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	return s.completeSheinRecoveredRemoteSuccess(ctx, state)
}

func (s *taskSubmissionRecoveryService) shouldRecoverLocally(state *sheinRecoveredRemoteState) bool {
	if state == nil {
		return false
	}
	return sheinpub.SubmissionResponseAcceptedForAction(state.completion.action, state.completion.response)
}

type sheinSubmitStartedAtContextKey struct{}

func (s *taskSubmissionRecoveryService) beginSheinSubmitLeaseWithoutStartTime(ctx context.Context, taskID, action, requestID string) (*Task, error) {
	startedAt, _ := ctx.Value(sheinSubmitStartedAtContextKey{}).(time.Time)
	return s.beginSheinSubmitLease(ctx, taskID, action, requestID, startedAt)
}

func (s *taskSubmissionRecoveryService) buildSheinReplayPreview(ctx context.Context, task *Task) (*ListingKitPreview, error) {
	return s.buildTaskPreview(ctx, task, "shein")
}

func (s *taskSubmissionRecoveryService) buildMissingPackageAcquireError(error) error {
	return fmt.Errorf("%w: shein preview payload is not available", ErrSubmitBlocked)
}

func (s *taskSubmissionRecoveryService) recordWorkflowStartFailure(ctx context.Context, in sheinWorkflowStartFailureInput) error {
	var result *ListingKitResult
	var pkg *SheinPackage
	if in.task != nil {
		result = in.task.Result
		if in.task.Result != nil {
			pkg = in.task.Result.Shein
		}
	}
	if s.recordSubmissionFailure == nil {
		return in.startErr
	}
	return s.recordSubmissionFailure(
		ctx,
		in.taskID,
		result,
		pkg,
		in.action,
		in.requestID,
		sheinpub.SubmissionPhaseValidate,
		in.startErr,
	)
}

func (s *taskSubmissionRecoveryService) clearWorkflowStartFailure(ctx context.Context, in sheinWorkflowStartFailureInput) error {
	return s.clearSheinSubmitLeaseAfterStartFailure(ctx, in.taskID, in.action, in.requestID, in.startErr)
}

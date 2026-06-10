package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/listingkit/core"
	"task-processor/internal/listingkit/submission"
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
	resolveRemoteStatusCallback func(sheinproduct.ProductAPI, sheinother.OtherAPI, string, string, []string, string, bool, string, time.Time, string) (*sheinRemoteConfirmation, error)
}

type taskSubmissionRecoveryService struct {
	repo                        Repository
	buildTaskPreview            func(context.Context, *Task, string) (*ListingKitPreview, error)
	buildSheinSubmitProductAPI  func(context.Context, *Task) (sheinproduct.ProductAPI, error)
	buildSheinSubmitOtherAPI    func(context.Context, *Task) (sheinother.OtherAPI, error)
	rememberSheinSubmitted      func(*Task, string)
	persistSuccessfulSubmission func(context.Context, string, *Task, string) error
	resolveRemoteStatusCallback func(sheinproduct.ProductAPI, sheinother.OtherAPI, string, string, []string, string, bool, string, time.Time, string) (*sheinRemoteConfirmation, error)
}

type sheinRecoveredRemoteState struct {
	report    *sheinpub.SubmissionReport
	record    *sheinpub.SubmissionRecord
	requestID string
	now       time.Time
	response  *sheinpub.SubmissionResponse
}

func newTaskSubmissionRecoveryService(config taskSubmissionRecoveryServiceConfig) *taskSubmissionRecoveryService {
	return &taskSubmissionRecoveryService{
		repo:                        config.repo,
		buildTaskPreview:            config.buildTaskPreview,
		buildSheinSubmitProductAPI:  config.buildSheinSubmitProductAPI,
		buildSheinSubmitOtherAPI:    config.buildSheinSubmitOtherAPI,
		rememberSheinSubmitted:      config.rememberSheinSubmitted,
		persistSuccessfulSubmission: config.persistSuccessfulSubmission,
		resolveRemoteStatusCallback: config.resolveRemoteStatusCallback,
	}
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

func (s *taskSubmissionRecoveryService) recoverSheinSubmitRemote(ctx context.Context, task *Task, action string) (*ListingKitPreview, error) {
	pkg, report, err := loadRecoveredSheinSubmissionReport(task)
	if err != nil {
		return nil, err
	}
	recoveredState := buildRecoveredSheinRemoteState(report, action)
	return s.executeRecoveredSheinSubmitRoute(ctx, task, pkg, action, recoveredState)
}

func (s *taskSubmissionRecoveryService) executeRecoveredSheinSubmitRoute(ctx context.Context, task *Task, pkg *SheinPackage, action string, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if sheinSubmissionResponseAccepted(state.response) || (action == "save_draft" && submission.SaveDraftSucceeded(action, state.response)) {
		return s.recoverSheinSubmitLocally(ctx, task, pkg, action, state)
	}
	return s.recoverSheinSubmitViaRemoteConfirmation(ctx, task, pkg, action, state)
}

func (s *taskSubmissionRecoveryService) recoverSheinSubmitLocally(ctx context.Context, task *Task, pkg *SheinPackage, action string, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if task == nil || pkg == nil || state == nil {
		return nil, ErrTaskResultUnavailable
	}
	appendSheinSubmissionEvent(pkg, submission.BuildPhaseEvent(task.ID, action, sheinpub.SubmissionPhasePersistResult, sheinpub.SubmissionStatusRunning, state.requestID, state.now, "恢复本地提交完成状态", nil))
	if state.record != nil {
		_, completionEvent := submission.CompleteAttemptAndBuildEvent(pkg, task.ID, action, state.requestID, state.response, nil, state.record.StartedAt, time.Now())
		appendSheinSubmissionEvent(pkg, completionEvent)
	}
	return s.finalizeRecoveredSheinSubmission(ctx, task, action)
}

func loadRecoveredSheinSubmissionReport(task *Task) (*SheinPackage, *sheinpub.SubmissionReport, error) {
	if task == nil || task.Result == nil {
		return nil, nil, ErrTaskResultUnavailable
	}
	pkg := sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
	if pkg == nil || pkg.SubmissionState == nil {
		return nil, nil, ErrTaskResultUnavailable
	}
	return pkg, pkg.SubmissionState, nil
}

func buildRecoveredSheinRemoteState(report *sheinpub.SubmissionReport, action string) *sheinRecoveredRemoteState {
	if report == nil {
		return nil
	}
	record := sheinSubmissionRecordForAction(report, action)
	response := recordResult(record)
	if response == nil {
		response = report.LastResult
	}
	return &sheinRecoveredRemoteState{
		report:    report,
		record:    record,
		requestID: report.CurrentRequestID,
		now:       time.Now(),
		response:  response,
	}
}

func (s *taskSubmissionRecoveryService) recoverSheinSubmitViaRemoteConfirmation(ctx context.Context, task *Task, pkg *SheinPackage, action string, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if task == nil || pkg == nil || state == nil {
		return nil, ErrTaskResultUnavailable
	}
	if state.record == nil || strings.TrimSpace(state.record.SupplierCode) == "" {
		return nil, fmt.Errorf("%w: stale SHEIN submit has no supplier code", core.ErrSubmitInProgress)
	}
	productAPI, err := s.buildSheinSubmitProductAPI(ctx, task)
	if err != nil {
		return nil, err
	}
	appendSheinSubmissionEvent(pkg, submission.AdvancePhaseAndBuildEvent(pkg, task.ID, action, state.requestID, sheinpub.SubmissionPhaseConfirmRemote, state.now, sheinSubmitInFlightTTL))
	if state.record == nil {
		return nil, ErrTaskResultUnavailable
	}
	event, remoteErr := s.refreshSheinSubmitRemoteStatus(ctx, task, task.ID, pkg, productAPI, action, state.requestID, state.record.SupplierCode, state.now)
	if pkg != nil && event != nil {
		appendSheinSubmissionEvent(pkg, *event)
	}
	if remoteErr != nil {
		return nil, s.persistSheinRecoveredRemoteFailure(ctx, task, pkg, action, state, remoteErr)
	}
	return s.completeSheinRecoveredRemoteSuccess(ctx, task, pkg, action, state)
}

func (s *taskSubmissionRecoveryService) persistSheinRecoveredRemoteFailure(ctx context.Context, task *Task, pkg *SheinPackage, action string, state *sheinRecoveredRemoteState, remoteErr error) error {
	if task == nil || pkg == nil || state == nil {
		return remoteErr
	}
	_, failureEvent := submission.FailAttemptAndBuildEvent(pkg, task.ID, action, state.requestID, sheinpub.SubmissionPhaseConfirmRemote, remoteErr, time.Now())
	appendSheinSubmissionEvent(pkg, failureEvent)
	if task.Result == nil {
		return remoteErr
	}
	task.Result.UpdatedAt = time.Now()
	if err := s.repo.SaveTaskResult(ctx, task.ID, task.Result); err != nil {
		return err
	}
	return remoteErr
}

func (s *taskSubmissionRecoveryService) completeSheinRecoveredRemoteSuccess(ctx context.Context, task *Task, pkg *SheinPackage, action string, state *sheinRecoveredRemoteState) (*ListingKitPreview, error) {
	if task == nil || pkg == nil || state == nil {
		return nil, ErrTaskResultUnavailable
	}
	if state.record != nil {
		response := state.record.Result
		if response == nil && state.report != nil {
			response = state.report.LastResult
		}
		record, completionEvent := submission.CompleteAttemptAndBuildEvent(pkg, task.ID, action, state.requestID, response, nil, state.record.StartedAt, time.Now())
		if record != nil && record.Result == nil {
			record.Status = sheinpub.SubmissionStatusSuccess
		}
		appendSheinSubmissionEvent(pkg, completionEvent)
	}
	return s.finalizeRecoveredSheinSubmission(ctx, task, action)
}

func (s *taskSubmissionRecoveryService) finalizeRecoveredSheinSubmission(ctx context.Context, task *Task, action string) (*ListingKitPreview, error) {
	if task == nil {
		return nil, ErrTaskResultUnavailable
	}
	if s.rememberSheinSubmitted != nil {
		s.rememberSheinSubmitted(task, action)
	}
	if err := s.persistSuccessfulSubmission(ctx, task.ID, task, action); err != nil {
		return nil, err
	}
	return s.buildTaskPreview(ctx, task, "shein")
}

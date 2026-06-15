package listingkit

import (
	"context"

	submissiondomain "task-processor/internal/listing/submission"
	sheinpub "task-processor/internal/publishing/shein"
)

type taskTemporalSubmissionPersistenceServiceConfig struct {
	loadSheinPublishTask                 func(context.Context, string) (*Task, *SheinPackage, error)
	saveTaskResult                       func(context.Context, string, *ListingKitResult) error
	persistSheinSubmitPhase              func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string) error
	persistSuccessfulSheinSubmission     func(context.Context, string, *Task, string) error
	recordSheinSubmissionFailureForState func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string, error) error
	rememberSheinSubmitted               func(*Task, string)
	successRunner                        *submissiondomain.SuccessPersistenceService[*Task, *SheinPackage, *sheinpub.SubmissionResponse]
	failureRunner                        *submissiondomain.FailurePersistenceService[*ListingKitResult, *SheinPackage]
}

type taskTemporalSubmissionPersistenceService struct {
	loadSheinPublishTask                 func(context.Context, string) (*Task, *SheinPackage, error)
	saveTaskResult                       func(context.Context, string, *ListingKitResult) error
	persistSheinSubmitPhase              func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string) error
	persistSuccessfulSheinSubmission     func(context.Context, string, *Task, string) error
	recordSheinSubmissionFailureForState func(context.Context, string, *ListingKitResult, *SheinPackage, string, string, string, error) error
	rememberSheinSubmitted               func(*Task, string)
	resultRunner                         *submissiondomain.ResultPersistenceService[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]
}

type sheinTemporalSubmissionPersistenceState struct {
	completion   sheinRemoteCompletionState
	phase        string
	errorMessage string
}

func newTaskTemporalSubmissionPersistenceService(config taskTemporalSubmissionPersistenceServiceConfig) *taskTemporalSubmissionPersistenceService {
	service := &taskTemporalSubmissionPersistenceService{
		loadSheinPublishTask:                 config.loadSheinPublishTask,
		saveTaskResult:                       config.saveTaskResult,
		persistSheinSubmitPhase:              config.persistSheinSubmitPhase,
		persistSuccessfulSheinSubmission:     config.persistSuccessfulSheinSubmission,
		recordSheinSubmissionFailureForState: config.recordSheinSubmissionFailureForState,
		rememberSheinSubmitted:               config.rememberSheinSubmitted,
	}
	successRunner := config.successRunner
	if successRunner == nil {
		successRunner = newSheinSubmissionSuccessPersistenceService(
			service.completeTemporalSubmitAttempt,
			service.persistTemporalSuccessResultAndPhase,
			service.rememberSheinSubmitted,
			service.persistSuccessfulSheinSubmission,
		)
	}
	failureRunner := config.failureRunner
	if failureRunner == nil {
		failureRunner = newSheinSubmissionFailurePersistenceService(service.recordTemporalFailureState)
	}
	service.resultRunner = submissiondomain.NewResultPersistenceService(submissiondomain.ResultPersistenceServiceConfig[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]{
		SuccessRunner: successRunner,
		FailureRunner: failureRunner,
		BuildSuccessInput: func(in submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]) submissiondomain.SuccessPersistenceInput[*Task, *SheinPackage, *sheinpub.SubmissionResponse] {
			return submissiondomain.BuildSuccessPersistenceInput(in)
		},
		BuildFailureInput: func(in submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]) submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage] {
			return submissiondomain.BuildFailurePersistenceInput(in)
		},
		FallbackSuccess: func(ctx context.Context, in submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]) error {
			return service.persistTemporalSuccessFallback(ctx, in)
		},
		FallbackFailure: func(ctx context.Context, in submissiondomain.ResultPersistenceInput[*Task, *ListingKitResult, *SheinPackage, *sheinpub.SubmissionResponse]) error {
			return service.recordTemporalFailureState(ctx, submissiondomain.FailurePersistenceInput[*ListingKitResult, *SheinPackage]{
				TaskID:    in.TaskID,
				Result:    in.Result,
				Package:   in.Package,
				Action:    in.Action,
				RequestID: in.RequestID,
				Phase:     in.Phase,
				Err:       in.Err,
			})
		},
	})
	return service
}

func (s *taskTemporalSubmissionPersistenceService) PersistSheinPublishSuccess(ctx context.Context, in SheinPersistSubmitSuccessInput) error {
	state, err := s.loadSheinSubmitPersistenceState(ctx, in.TaskID, in.Action, in.RequestID, in.SupplierCode, in.Response, in.Snapshot, "", "")
	if err != nil {
		return err
	}
	return s.persistSheinTemporalSubmissionSuccess(ctx, state)
}

func (s *taskTemporalSubmissionPersistenceService) PersistSheinPublishFailure(ctx context.Context, in SheinPersistSubmitFailureInput) error {
	state, err := s.loadSheinSubmitPersistenceState(ctx, in.TaskID, in.Action, in.RequestID, in.SupplierCode, in.Response, in.Snapshot, in.Phase, in.ErrorMessage)
	if err != nil {
		return err
	}
	return s.persistSheinTemporalSubmissionFailure(ctx, state)
}

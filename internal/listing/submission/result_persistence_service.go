package submission

import (
	"context"
	"time"
)

type ResultPersistenceInput[TTask, TResult, TPackage, TResponse any] struct {
	TaskID    string
	Task      TTask
	Result    TResult
	Package   TPackage
	Action    string
	RequestID string
	Phase     string
	Response  TResponse
	StartedAt time.Time
	Err       error
}

type ResultPersistenceService[TTask, TResult, TPackage, TResponse any] struct {
	successRunner         *SuccessPersistenceService[TTask, TPackage, TResponse]
	failureRunner         *FailurePersistenceService[TResult, TPackage]
	buildSuccessInput     func(ResultPersistenceInput[TTask, TResult, TPackage, TResponse]) SuccessPersistenceInput[TTask, TPackage, TResponse]
	buildFailureInput     func(ResultPersistenceInput[TTask, TResult, TPackage, TResponse]) FailurePersistenceInput[TResult, TPackage]
	beforeFailure         func(ResultPersistenceInput[TTask, TResult, TPackage, TResponse])
	fallbackSuccess       func(context.Context, ResultPersistenceInput[TTask, TResult, TPackage, TResponse]) error
	fallbackFailure       func(context.Context, ResultPersistenceInput[TTask, TResult, TPackage, TResponse]) error
	returnOriginalFailure bool
}

type ResultPersistenceServiceConfig[TTask, TResult, TPackage, TResponse any] struct {
	SuccessRunner         *SuccessPersistenceService[TTask, TPackage, TResponse]
	FailureRunner         *FailurePersistenceService[TResult, TPackage]
	BuildSuccessInput     func(ResultPersistenceInput[TTask, TResult, TPackage, TResponse]) SuccessPersistenceInput[TTask, TPackage, TResponse]
	BuildFailureInput     func(ResultPersistenceInput[TTask, TResult, TPackage, TResponse]) FailurePersistenceInput[TResult, TPackage]
	BeforeFailure         func(ResultPersistenceInput[TTask, TResult, TPackage, TResponse])
	FallbackSuccess       func(context.Context, ResultPersistenceInput[TTask, TResult, TPackage, TResponse]) error
	FallbackFailure       func(context.Context, ResultPersistenceInput[TTask, TResult, TPackage, TResponse]) error
	ReturnOriginalFailure bool
}

func NewResultPersistenceService[TTask, TResult, TPackage, TResponse any](config ResultPersistenceServiceConfig[TTask, TResult, TPackage, TResponse]) *ResultPersistenceService[TTask, TResult, TPackage, TResponse] {
	return &ResultPersistenceService[TTask, TResult, TPackage, TResponse]{
		successRunner:         config.SuccessRunner,
		failureRunner:         config.FailureRunner,
		buildSuccessInput:     config.BuildSuccessInput,
		buildFailureInput:     config.BuildFailureInput,
		beforeFailure:         config.BeforeFailure,
		fallbackSuccess:       config.FallbackSuccess,
		fallbackFailure:       config.FallbackFailure,
		returnOriginalFailure: config.ReturnOriginalFailure,
	}
}

func (s *ResultPersistenceService[TTask, TResult, TPackage, TResponse]) PersistSuccess(ctx context.Context, in ResultPersistenceInput[TTask, TResult, TPackage, TResponse]) error {
	if s == nil {
		return nil
	}
	if s.successRunner != nil && s.buildSuccessInput != nil {
		return s.successRunner.PersistSuccess(ctx, s.buildSuccessInput(in))
	}
	if s.fallbackSuccess != nil {
		return s.fallbackSuccess(ctx, in)
	}
	return nil
}

func (s *ResultPersistenceService[TTask, TResult, TPackage, TResponse]) PersistFailure(ctx context.Context, in ResultPersistenceInput[TTask, TResult, TPackage, TResponse]) error {
	if s == nil {
		return nil
	}
	if s.beforeFailure != nil {
		s.beforeFailure(in)
	}
	if s.failureRunner != nil && s.buildFailureInput != nil {
		if err := s.failureRunner.PersistFailure(ctx, s.buildFailureInput(in)); err != nil {
			return err
		}
		if s.returnOriginalFailure {
			return in.Err
		}
		return nil
	}
	if s.fallbackFailure != nil {
		if err := s.fallbackFailure(ctx, in); err != nil {
			return err
		}
		if s.returnOriginalFailure {
			return in.Err
		}
		return nil
	}
	if s.returnOriginalFailure {
		return in.Err
	}
	return nil
}

func (s *ResultPersistenceService[TTask, TResult, TPackage, TResponse]) Finish(ctx context.Context, in ResultPersistenceInput[TTask, TResult, TPackage, TResponse]) error {
	if in.Err != nil {
		return s.PersistFailure(ctx, in)
	}
	return s.PersistSuccess(ctx, in)
}

package submission

import (
	"context"
	"time"
)

type SuccessPersistenceInput[TTask, TPackage, TResponse any] struct {
	TaskID    string
	Task      TTask
	Package   TPackage
	Action    string
	RequestID string
	Response  TResponse
	StartedAt time.Time
}

type SuccessPersistenceService[TTask, TPackage, TResponse any] struct {
	persistResultAndPhase       func(context.Context, SuccessPersistenceInput[TTask, TPackage, TResponse]) error
	completeAttempt             func(SuccessPersistenceInput[TTask, TPackage, TResponse], time.Time)
	rememberSubmitted           func(TTask, string)
	persistSuccessfulSubmission func(context.Context, string, TTask, string) error
	currentTime                 func() time.Time
}

type SuccessPersistenceServiceConfig[TTask, TPackage, TResponse any] struct {
	PersistResultAndPhase       func(context.Context, SuccessPersistenceInput[TTask, TPackage, TResponse]) error
	CompleteAttempt             func(SuccessPersistenceInput[TTask, TPackage, TResponse], time.Time)
	RememberSubmitted           func(TTask, string)
	PersistSuccessfulSubmission func(context.Context, string, TTask, string) error
	CurrentTime                 func() time.Time
}

func NewSuccessPersistenceService[TTask, TPackage, TResponse any](config SuccessPersistenceServiceConfig[TTask, TPackage, TResponse]) *SuccessPersistenceService[TTask, TPackage, TResponse] {
	return &SuccessPersistenceService[TTask, TPackage, TResponse]{
		persistResultAndPhase:       config.PersistResultAndPhase,
		completeAttempt:             config.CompleteAttempt,
		rememberSubmitted:           config.RememberSubmitted,
		persistSuccessfulSubmission: config.PersistSuccessfulSubmission,
		currentTime:                 config.CurrentTime,
	}
}

func (s *SuccessPersistenceService[TTask, TPackage, TResponse]) PersistSuccess(ctx context.Context, in SuccessPersistenceInput[TTask, TPackage, TResponse]) error {
	if s == nil {
		return nil
	}
	if s.persistResultAndPhase != nil {
		if err := s.persistResultAndPhase(ctx, in); err != nil {
			return err
		}
	}
	finishedAt := time.Now()
	if s.currentTime != nil {
		finishedAt = s.currentTime()
	}
	if s.completeAttempt != nil {
		s.completeAttempt(in, finishedAt)
	}
	if s.rememberSubmitted != nil {
		s.rememberSubmitted(in.Task, in.Action)
	}
	if s.persistSuccessfulSubmission != nil {
		return s.persistSuccessfulSubmission(ctx, in.TaskID, in.Task, in.Action)
	}
	return nil
}

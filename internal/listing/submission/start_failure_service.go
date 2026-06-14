package submission

import (
	"context"
	"errors"
)

type StartFailureService[Input any] struct {
	recordFailure func(context.Context, Input) error
	clearFailure  func(context.Context, Input) error
	originalError func(Input) error
}

type StartFailureServiceConfig[Input any] struct {
	RecordFailure func(context.Context, Input) error
	ClearFailure  func(context.Context, Input) error
	OriginalError func(Input) error
}

func NewStartFailureService[Input any](config StartFailureServiceConfig[Input]) *StartFailureService[Input] {
	return &StartFailureService[Input]{
		recordFailure: config.RecordFailure,
		clearFailure:  config.ClearFailure,
		originalError: config.OriginalError,
	}
}

func (s *StartFailureService[Input]) Handle(ctx context.Context, in Input) error {
	if s == nil || s.originalError == nil {
		return nil
	}
	originalErr := s.originalError(in)
	failErr := originalErr
	if s.recordFailure != nil {
		failErr = s.recordFailure(ctx, in)
	}
	var clearErr error
	if s.clearFailure != nil {
		clearErr = s.clearFailure(ctx, in)
	}
	if failErr != nil && !errors.Is(failErr, originalErr) {
		if clearErr != nil {
			return errors.Join(failErr, clearErr)
		}
		return failErr
	}
	if clearErr != nil {
		return clearErr
	}
	return originalErr
}

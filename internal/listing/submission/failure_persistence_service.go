package submission

import "context"

type FailurePersistenceInput[TResult, TPackage any] struct {
	TaskID    string
	Result    TResult
	Package   TPackage
	Action    string
	RequestID string
	Phase     string
	Err       error
}

type FailurePersistenceService[TResult, TPackage any] struct {
	recordFailure func(context.Context, FailurePersistenceInput[TResult, TPackage]) error
}

type FailurePersistenceServiceConfig[TResult, TPackage any] struct {
	RecordFailure func(context.Context, FailurePersistenceInput[TResult, TPackage]) error
}

func NewFailurePersistenceService[TResult, TPackage any](config FailurePersistenceServiceConfig[TResult, TPackage]) *FailurePersistenceService[TResult, TPackage] {
	return &FailurePersistenceService[TResult, TPackage]{
		recordFailure: config.RecordFailure,
	}
}

func (s *FailurePersistenceService[TResult, TPackage]) PersistFailure(ctx context.Context, in FailurePersistenceInput[TResult, TPackage]) error {
	if s == nil || s.recordFailure == nil {
		return nil
	}
	return s.recordFailure(ctx, in)
}

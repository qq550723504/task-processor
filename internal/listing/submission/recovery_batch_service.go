package submission

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type RecoveryBatchRequest struct {
	DueBefore time.Time
	RecoverAt time.Time
	Limit     int
}

type RecoveryBatchService[T any] struct {
	listCandidates       func(context.Context, time.Time, int) ([]T, error)
	currentSubmitter     func() RecoverySubmitFunc
	markRecovered        func(context.Context, string, time.Time) error
	submitRecovered      func(context.Context, RecoverySubmitFunc, T, time.Time) error
	taskID               func(T) string
	isTaskNotRecoverable func(error) bool
	now                  func() time.Time
	errUnavailable       error
}

type RecoveryBatchServiceConfig[T any] struct {
	ListCandidates       func(context.Context, time.Time, int) ([]T, error)
	CurrentSubmitter     func() RecoverySubmitFunc
	MarkRecovered        func(context.Context, string, time.Time) error
	SubmitRecovered      func(context.Context, RecoverySubmitFunc, T, time.Time) error
	TaskID               func(T) string
	IsTaskNotRecoverable func(error) bool
	Now                  func() time.Time
	ErrUnavailable       error
}

func NewRecoveryBatchService[T any](config RecoveryBatchServiceConfig[T]) *RecoveryBatchService[T] {
	return &RecoveryBatchService[T]{
		listCandidates:       config.ListCandidates,
		currentSubmitter:     config.CurrentSubmitter,
		markRecovered:        config.MarkRecovered,
		submitRecovered:      config.SubmitRecovered,
		taskID:               config.TaskID,
		isTaskNotRecoverable: config.IsTaskNotRecoverable,
		now:                  config.Now,
		errUnavailable:       config.ErrUnavailable,
	}
}

func (s *RecoveryBatchService[T]) RecoverBatch(ctx context.Context, req *RecoveryBatchRequest) (int64, error) {
	if s == nil || s.listCandidates == nil {
		return 0, fmt.Errorf("task recovery candidate loader is not configured")
	}
	submitter := s.currentSubmitterOrNil()
	if submitter == nil {
		return 0, s.errUnavailable
	}

	recoverAt := s.currentTime()
	dueBefore := time.Time{}
	limit := 0
	if req != nil {
		dueBefore = req.DueBefore
		limit = req.Limit
		if !req.RecoverAt.IsZero() {
			recoverAt = req.RecoverAt
		}
	}
	if dueBefore.IsZero() {
		dueBefore = recoverAt
	}

	tasks, err := s.listCandidates(ctx, dueBefore, limit)
	if err != nil {
		return 0, err
	}

	var recovered int64
	var runErr error
	for _, task := range tasks {
		taskID := s.taskID(task)
		if err := s.markRecovered(ctx, taskID, recoverAt); err != nil {
			if s.isTaskNotRecoverable != nil && s.isTaskNotRecoverable(err) {
				continue
			}
			return recovered, err
		}
		if err := s.submitRecovered(ctx, submitter, task, recoverAt); err != nil {
			runErr = errors.Join(runErr, err)
			continue
		}
		recovered++
	}
	return recovered, runErr
}

func (s *RecoveryBatchService[T]) currentSubmitterOrNil() RecoverySubmitFunc {
	if s == nil || s.currentSubmitter == nil {
		return nil
	}
	return s.currentSubmitter()
}

func (s *RecoveryBatchService[T]) currentTime() time.Time {
	if s == nil || s.now == nil {
		return time.Now().UTC()
	}
	return s.now().UTC()
}

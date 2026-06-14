package submission

import (
	"context"
	"fmt"
	"strings"
)

type RecoverySubmitFunc func(string) error

type RecoveryNowService[T any] struct {
	loadTask         func(context.Context, string) (*T, error)
	currentSubmitter func() RecoverySubmitFunc
	markRecovered    func(context.Context, string) error
	submitRecovered  func(context.Context, RecoverySubmitFunc, string, *T) error
	reloadTask       func(context.Context, string) (*T, error)
	errUnavailable   error
	errEmptyTaskID   error
}

type RecoveryNowServiceConfig[T any] struct {
	LoadTask         func(context.Context, string) (*T, error)
	CurrentSubmitter func() RecoverySubmitFunc
	MarkRecovered    func(context.Context, string) error
	SubmitRecovered  func(context.Context, RecoverySubmitFunc, string, *T) error
	ReloadTask       func(context.Context, string) (*T, error)
	ErrUnavailable   error
	ErrEmptyTaskID   error
}

func NewRecoveryNowService[T any](config RecoveryNowServiceConfig[T]) *RecoveryNowService[T] {
	return &RecoveryNowService[T]{
		loadTask:         config.LoadTask,
		currentSubmitter: config.CurrentSubmitter,
		markRecovered:    config.MarkRecovered,
		submitRecovered:  config.SubmitRecovered,
		reloadTask:       config.ReloadTask,
		errUnavailable:   config.ErrUnavailable,
		errEmptyTaskID:   config.ErrEmptyTaskID,
	}
}

func (s *RecoveryNowService[T]) RecoverNow(ctx context.Context, taskID string) (*T, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, s.errEmptyTaskID
	}
	if s == nil || s.loadTask == nil {
		return nil, fmt.Errorf("task recovery loader is not configured")
	}
	submitter := s.currentSubmitterOrNil()
	if submitter == nil {
		return nil, s.errUnavailable
	}
	current, err := s.loadTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if err := s.markRecovered(ctx, taskID); err != nil {
		return nil, err
	}
	if err := s.submitRecovered(ctx, submitter, taskID, current); err != nil {
		return nil, err
	}
	return s.reloadTask(ctx, taskID)
}

func (s *RecoveryNowService[T]) currentSubmitterOrNil() RecoverySubmitFunc {
	if s == nil || s.currentSubmitter == nil {
		return nil
	}
	return s.currentSubmitter()
}

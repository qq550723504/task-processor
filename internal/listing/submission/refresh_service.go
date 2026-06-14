package submission

import "context"

// StatusRefreshService owns the generic orchestration for "refresh current
// submission state and finalize a refreshed response" style flows.
type StatusRefreshService[State any, Confirmation any, Result any] struct {
	lockKeySuffix       string
	lockSubmit          func(string) func()
	loadState           func(context.Context, string) (*State, error)
	resolveConfirmation func(string, *State) (*Confirmation, error)
	finish              func(context.Context, string, *State, *Confirmation, error) (*Result, error)
}

type StatusRefreshServiceConfig[State any, Confirmation any, Result any] struct {
	LockKeySuffix       string
	LockSubmit          func(string) func()
	LoadState           func(context.Context, string) (*State, error)
	ResolveConfirmation func(string, *State) (*Confirmation, error)
	Finish              func(context.Context, string, *State, *Confirmation, error) (*Result, error)
}

func NewStatusRefreshService[State any, Confirmation any, Result any](
	config StatusRefreshServiceConfig[State, Confirmation, Result],
) *StatusRefreshService[State, Confirmation, Result] {
	return &StatusRefreshService[State, Confirmation, Result]{
		lockKeySuffix:       config.LockKeySuffix,
		lockSubmit:          config.LockSubmit,
		loadState:           config.LoadState,
		resolveConfirmation: config.ResolveConfirmation,
		finish:              config.Finish,
	}
}

func (s *StatusRefreshService[State, Confirmation, Result]) RefreshStatus(
	ctx context.Context,
	taskID string,
) (*Result, error) {
	if s == nil {
		return nil, nil
	}
	if s.lockSubmit != nil {
		unlock := s.lockSubmit(s.lockKey(taskID))
		defer unlock()
	}
	state, err := s.loadState(ctx, taskID)
	if err != nil {
		return nil, err
	}
	confirmation, resolveErr := s.resolveConfirmation(taskID, state)
	if resolveErr != nil && confirmation == nil {
		return s.finish(ctx, taskID, state, nil, resolveErr)
	}
	return s.finish(ctx, taskID, state, confirmation, resolveErr)
}

func (s *StatusRefreshService[State, Confirmation, Result]) lockKey(taskID string) string {
	if s == nil || s.lockKeySuffix == "" {
		return taskID
	}
	return taskID + ":" + s.lockKeySuffix
}

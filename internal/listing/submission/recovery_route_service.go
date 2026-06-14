package submission

import (
	"context"
	"errors"
)

type RecoveryRouteService[State any, Result any] struct {
	useLocal      func(*State) bool
	recoverLocal  func(context.Context, *State) (*Result, error)
	recoverRemote func(context.Context, *State) (*Result, error)
}

type RecoveryRouteServiceConfig[State any, Result any] struct {
	UseLocal      func(*State) bool
	RecoverLocal  func(context.Context, *State) (*Result, error)
	RecoverRemote func(context.Context, *State) (*Result, error)
}

func NewRecoveryRouteService[State any, Result any](config RecoveryRouteServiceConfig[State, Result]) *RecoveryRouteService[State, Result] {
	return &RecoveryRouteService[State, Result]{
		useLocal:      config.UseLocal,
		recoverLocal:  config.RecoverLocal,
		recoverRemote: config.RecoverRemote,
	}
}

func (s *RecoveryRouteService[State, Result]) Recover(ctx context.Context, state *State) (*Result, error) {
	if s == nil {
		return nil, nil
	}
	if state == nil {
		return nil, errors.New("recovery route state is not configured")
	}
	if s.useLocal == nil || s.recoverLocal == nil || s.recoverRemote == nil {
		return nil, errors.New("recovery route service is incomplete")
	}
	if s.useLocal(state) {
		return s.recoverLocal(ctx, state)
	}
	return s.recoverRemote(ctx, state)
}

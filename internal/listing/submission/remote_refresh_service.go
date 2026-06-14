package submission

import (
	"context"
	"errors"
	"fmt"
)

type RemoteRefreshService[State any, Request any, Event any, Result any] struct {
	persistPhase func(context.Context, *State) error
	buildRequest func(context.Context, *State) (Request, error)
	execute      func(context.Context, Request) (Event, error)
	recordEvent  func(*State, Event)
	finishError  func(context.Context, *State, error) (*Result, error)
	finishOK     func(context.Context, *State) (*Result, error)
}

type RemoteRefreshServiceConfig[State any, Request any, Event any, Result any] struct {
	PersistPhase func(context.Context, *State) error
	BuildRequest func(context.Context, *State) (Request, error)
	Execute      func(context.Context, Request) (Event, error)
	RecordEvent  func(*State, Event)
	FinishError  func(context.Context, *State, error) (*Result, error)
	FinishOK     func(context.Context, *State) (*Result, error)
}

func NewRemoteRefreshService[State any, Request any, Event any, Result any](config RemoteRefreshServiceConfig[State, Request, Event, Result]) *RemoteRefreshService[State, Request, Event, Result] {
	return &RemoteRefreshService[State, Request, Event, Result]{
		persistPhase: config.PersistPhase,
		buildRequest: config.BuildRequest,
		execute:      config.Execute,
		recordEvent:  config.RecordEvent,
		finishError:  config.FinishError,
		finishOK:     config.FinishOK,
	}
}

func (s *RemoteRefreshService[State, Request, Event, Result]) Refresh(ctx context.Context, state *State) (*Result, error) {
	if s == nil {
		return nil, nil
	}
	if state == nil {
		return nil, fmt.Errorf("remote refresh state is not configured")
	}
	if s.buildRequest == nil || s.execute == nil || s.finishError == nil || s.finishOK == nil {
		return nil, errors.New("remote refresh service is incomplete")
	}
	if s.persistPhase != nil {
		if err := s.persistPhase(ctx, state); err != nil {
			return nil, err
		}
	}
	request, err := s.buildRequest(ctx, state)
	if err != nil {
		return nil, err
	}
	event, remoteErr := s.execute(ctx, request)
	if s.recordEvent != nil {
		s.recordEvent(state, event)
	}
	if remoteErr != nil {
		result, err := s.finishError(ctx, state, remoteErr)
		if err != nil {
			return nil, err
		}
		return result, remoteErr
	}
	return s.finishOK(ctx, state)
}

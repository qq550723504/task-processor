package studio

import (
	"context"
	"fmt"
	"time"
)

type BatchTaskResumeFinalizeState[Session any, Batch any, CreatedTask any, FailedTask any] struct {
	Session      *Session
	Batch        *Batch
	CreatedTasks []CreatedTask
	FailedTasks  []FailedTask
}

type BatchTaskResumeFinalizeService[Session any, Batch any, Result any, CreatedTask any, FailedTask any] struct {
	updateSession     func(context.Context, *Session) error
	clearPendingTasks func(*Session)
	setCreatedTasks   func(*Session, []CreatedTask)
	setFailedTasks    func(*Session, []FailedTask)
	setSessionDone    func(*Session)
	setSessionUpdated func(*Session, time.Time)
	updateBatch       func(context.Context, *Batch) error
	setBatchDone      func(*Batch)
	setBatchUpdated   func(*Batch, time.Time)
	loadResult        func(context.Context, string) (*Result, error)
	currentTime       func() time.Time
}

type BatchTaskResumeFinalizeServiceConfig[Session any, Batch any, Result any, CreatedTask any, FailedTask any] struct {
	UpdateSession     func(context.Context, *Session) error
	ClearPendingTasks func(*Session)
	SetCreatedTasks   func(*Session, []CreatedTask)
	SetFailedTasks    func(*Session, []FailedTask)
	SetSessionDone    func(*Session)
	SetSessionUpdated func(*Session, time.Time)
	UpdateBatch       func(context.Context, *Batch) error
	SetBatchDone      func(*Batch)
	SetBatchUpdated   func(*Batch, time.Time)
	LoadResult        func(context.Context, string) (*Result, error)
	CurrentTime       func() time.Time
}

func NewBatchTaskResumeFinalizeService[Session any, Batch any, Result any, CreatedTask any, FailedTask any](
	config BatchTaskResumeFinalizeServiceConfig[Session, Batch, Result, CreatedTask, FailedTask],
) *BatchTaskResumeFinalizeService[Session, Batch, Result, CreatedTask, FailedTask] {
	return &BatchTaskResumeFinalizeService[Session, Batch, Result, CreatedTask, FailedTask]{
		updateSession:     config.UpdateSession,
		clearPendingTasks: config.ClearPendingTasks,
		setCreatedTasks:   config.SetCreatedTasks,
		setFailedTasks:    config.SetFailedTasks,
		setSessionDone:    config.SetSessionDone,
		setSessionUpdated: config.SetSessionUpdated,
		updateBatch:       config.UpdateBatch,
		setBatchDone:      config.SetBatchDone,
		setBatchUpdated:   config.SetBatchUpdated,
		loadResult:        config.LoadResult,
		currentTime:       config.CurrentTime,
	}
}

func (s *BatchTaskResumeFinalizeService[Session, Batch, Result, CreatedTask, FailedTask]) FinalizeTaskCreation(
	ctx context.Context,
	batchID string,
	state BatchTaskResumeFinalizeState[Session, Batch, CreatedTask, FailedTask],
) (*Result, error) {
	if s == nil || s.loadResult == nil {
		return nil, fmt.Errorf("studio batch task resume finalize service is not configured")
	}
	if state.Session == nil || state.Batch == nil {
		return nil, fmt.Errorf("studio batch task resume finalize state is not configured")
	}
	now := time.Now().UTC()
	if s.currentTime != nil {
		now = s.currentTime().UTC()
	}

	if s.clearPendingTasks != nil {
		s.clearPendingTasks(state.Session)
	}
	if s.setCreatedTasks != nil {
		s.setCreatedTasks(state.Session, state.CreatedTasks)
	}
	if s.setFailedTasks != nil {
		s.setFailedTasks(state.Session, state.FailedTasks)
	}
	if s.setSessionDone != nil {
		s.setSessionDone(state.Session)
	}
	if s.setSessionUpdated != nil {
		s.setSessionUpdated(state.Session, now)
	}
	if s.updateSession != nil {
		if err := s.updateSession(ctx, state.Session); err != nil {
			return nil, err
		}
	}

	if s.setBatchDone != nil {
		s.setBatchDone(state.Batch)
	}
	if s.setBatchUpdated != nil {
		s.setBatchUpdated(state.Batch, now)
	}
	if s.updateBatch != nil {
		if err := s.updateBatch(ctx, state.Batch); err != nil {
			return nil, err
		}
	}

	return s.loadResult(ctx, batchID)
}

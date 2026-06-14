package studio

import (
	"context"
	"fmt"
	"time"
)

type BatchTaskPrepareState[Session any, Batch any] struct {
	Session   *Session
	Batch     *Batch
	DesignIDs []string
}

type BatchTaskPrepareService[Session any, Batch any, Result any] struct {
	updateSession       func(context.Context, *Session) error
	setPendingDesignIDs func(*Session, []string)
	clearFailedTasks    func(*Session)
	setSessionCreating  func(*Session)
	setSessionUpdatedAt func(*Session, time.Time)
	updateBatch         func(context.Context, *Batch) error
	setBatchCreating    func(*Batch)
	setBatchUpdatedAt   func(*Batch, time.Time)
	loadResult          func(context.Context, string) (*Result, error)
	currentTime         func() time.Time
}

type BatchTaskPrepareServiceConfig[Session any, Batch any, Result any] struct {
	UpdateSession       func(context.Context, *Session) error
	SetPendingDesignIDs func(*Session, []string)
	ClearFailedTasks    func(*Session)
	SetSessionCreating  func(*Session)
	SetSessionUpdatedAt func(*Session, time.Time)
	UpdateBatch         func(context.Context, *Batch) error
	SetBatchCreating    func(*Batch)
	SetBatchUpdatedAt   func(*Batch, time.Time)
	LoadResult          func(context.Context, string) (*Result, error)
	CurrentTime         func() time.Time
}

func NewBatchTaskPrepareService[Session any, Batch any, Result any](
	config BatchTaskPrepareServiceConfig[Session, Batch, Result],
) *BatchTaskPrepareService[Session, Batch, Result] {
	return &BatchTaskPrepareService[Session, Batch, Result]{
		updateSession:       config.UpdateSession,
		setPendingDesignIDs: config.SetPendingDesignIDs,
		clearFailedTasks:    config.ClearFailedTasks,
		setSessionCreating:  config.SetSessionCreating,
		setSessionUpdatedAt: config.SetSessionUpdatedAt,
		updateBatch:         config.UpdateBatch,
		setBatchCreating:    config.SetBatchCreating,
		setBatchUpdatedAt:   config.SetBatchUpdatedAt,
		loadResult:          config.LoadResult,
		currentTime:         config.CurrentTime,
	}
}

func (s *BatchTaskPrepareService[Session, Batch, Result]) PrepareTaskCreation(
	ctx context.Context,
	batchID string,
	state BatchTaskPrepareState[Session, Batch],
) (*Result, error) {
	if s == nil || s.loadResult == nil {
		return nil, fmt.Errorf("studio batch task prepare service is not configured")
	}
	if state.Session == nil || state.Batch == nil {
		return nil, fmt.Errorf("studio batch task prepare state is not configured")
	}
	now := time.Now().UTC()
	if s.currentTime != nil {
		now = s.currentTime().UTC()
	}

	if s.setPendingDesignIDs != nil {
		s.setPendingDesignIDs(state.Session, state.DesignIDs)
	}
	if s.clearFailedTasks != nil {
		s.clearFailedTasks(state.Session)
	}
	if s.setSessionCreating != nil {
		s.setSessionCreating(state.Session)
	}
	if s.setSessionUpdatedAt != nil {
		s.setSessionUpdatedAt(state.Session, now)
	}
	if s.updateSession != nil {
		if err := s.updateSession(ctx, state.Session); err != nil {
			return nil, err
		}
	}

	if s.setBatchCreating != nil {
		s.setBatchCreating(state.Batch)
	}
	if s.setBatchUpdatedAt != nil {
		s.setBatchUpdatedAt(state.Batch, now)
	}
	if s.updateBatch != nil {
		if err := s.updateBatch(ctx, state.Batch); err != nil {
			return nil, err
		}
	}

	return s.loadResult(ctx, batchID)
}

package studio

import (
	"context"
	"fmt"
	"strings"
)

type BatchTaskCreationService[Session any, Batch any, Result any, CreatedTask any, FailedTask any] struct {
	prepareState         func(context.Context, string, []string) (BatchTaskPrepareState[Session, Batch], error)
	prepareTaskCreation  func(context.Context, string, BatchTaskPrepareState[Session, Batch]) (*Result, error)
	loadSession          func(context.Context, string) (*Session, error)
	pendingDesignIDs     func(*Session) []string
	loadResult           func(context.Context, string) (*Result, error)
	createTasks          func(context.Context, string, []string) (*Result, error)
	loadBatch            func(context.Context, string) (*Batch, error)
	finalizeTaskCreation func(context.Context, string, BatchTaskResumeFinalizeState[Session, Batch, CreatedTask, FailedTask]) (*Result, error)
	createdTasks         func(*Result) []CreatedTask
	failedTasks          func(*Result) []FailedTask
}

type BatchTaskCreationServiceConfig[Session any, Batch any, Result any, CreatedTask any, FailedTask any] struct {
	PrepareState         func(context.Context, string, []string) (BatchTaskPrepareState[Session, Batch], error)
	PrepareTaskCreation  func(context.Context, string, BatchTaskPrepareState[Session, Batch]) (*Result, error)
	LoadSession          func(context.Context, string) (*Session, error)
	PendingDesignIDs     func(*Session) []string
	LoadResult           func(context.Context, string) (*Result, error)
	CreateTasks          func(context.Context, string, []string) (*Result, error)
	LoadBatch            func(context.Context, string) (*Batch, error)
	FinalizeTaskCreation func(context.Context, string, BatchTaskResumeFinalizeState[Session, Batch, CreatedTask, FailedTask]) (*Result, error)
	CreatedTasks         func(*Result) []CreatedTask
	FailedTasks          func(*Result) []FailedTask
}

func NewBatchTaskCreationService[Session any, Batch any, Result any, CreatedTask any, FailedTask any](
	config BatchTaskCreationServiceConfig[Session, Batch, Result, CreatedTask, FailedTask],
) *BatchTaskCreationService[Session, Batch, Result, CreatedTask, FailedTask] {
	return &BatchTaskCreationService[Session, Batch, Result, CreatedTask, FailedTask]{
		prepareState:         config.PrepareState,
		prepareTaskCreation:  config.PrepareTaskCreation,
		loadSession:          config.LoadSession,
		pendingDesignIDs:     config.PendingDesignIDs,
		loadResult:           config.LoadResult,
		createTasks:          config.CreateTasks,
		loadBatch:            config.LoadBatch,
		finalizeTaskCreation: config.FinalizeTaskCreation,
		createdTasks:         config.CreatedTasks,
		failedTasks:          config.FailedTasks,
	}
}

func (s *BatchTaskCreationService[Session, Batch, Result, CreatedTask, FailedTask]) PrepareTaskCreation(
	ctx context.Context,
	batchID string,
	designIDs []string,
) (*Result, error) {
	if s == nil || s.prepareState == nil || s.prepareTaskCreation == nil {
		return nil, fmt.Errorf("studio batch task creation service is not configured")
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	state, err := s.prepareState(ctx, normalizedBatchID, append([]string(nil), designIDs...))
	if err != nil {
		return nil, err
	}
	return s.prepareTaskCreation(ctx, normalizedBatchID, state)
}

func (s *BatchTaskCreationService[Session, Batch, Result, CreatedTask, FailedTask]) ResumeTaskCreation(
	ctx context.Context,
	batchID string,
) (*Result, error) {
	if s == nil || s.loadSession == nil || s.pendingDesignIDs == nil || s.loadResult == nil || s.createTasks == nil || s.loadBatch == nil || s.finalizeTaskCreation == nil || s.createdTasks == nil || s.failedTasks == nil {
		return nil, fmt.Errorf("studio batch task creation service is not configured")
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	session, err := s.loadSession(ctx, normalizedBatchID)
	if err != nil {
		return nil, err
	}
	designIDs := append([]string(nil), s.pendingDesignIDs(session)...)
	if len(designIDs) == 0 {
		return s.loadResult(ctx, normalizedBatchID)
	}
	result, err := s.createTasks(ctx, normalizedBatchID, designIDs)
	if err != nil {
		return nil, err
	}
	batch, err := s.loadBatch(ctx, normalizedBatchID)
	if err != nil {
		return nil, err
	}
	return s.finalizeTaskCreation(ctx, normalizedBatchID, BatchTaskResumeFinalizeState[Session, Batch, CreatedTask, FailedTask]{
		Session:      session,
		Batch:        batch,
		CreatedTasks: s.createdTasks(result),
		FailedTasks:  s.failedTasks(result),
	})
}

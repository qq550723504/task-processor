package studio

import (
	"context"
	"fmt"
	"strings"
)

type BatchTaskExecuteService[Session any, Candidate any, CreatedTask any, FailedTask any, Result any] struct {
	loadSession  func(context.Context, string) (*Session, error)
	loadItems    func(context.Context, string, []string) ([]Candidate, error)
	findExisting func(context.Context, *Session, Candidate) (CreatedTask, bool)
	createTask   func(context.Context, Candidate) (CreatedTask, error)
	buildFailed  func(Candidate, error) FailedTask
	finalize     func(context.Context, string, *Session, []CreatedTask, []FailedTask) (*Result, error)
}

type BatchTaskExecuteServiceConfig[Session any, Candidate any, CreatedTask any, FailedTask any, Result any] struct {
	LoadSession  func(context.Context, string) (*Session, error)
	LoadItems    func(context.Context, string, []string) ([]Candidate, error)
	FindExisting func(context.Context, *Session, Candidate) (CreatedTask, bool)
	CreateTask   func(context.Context, Candidate) (CreatedTask, error)
	BuildFailed  func(Candidate, error) FailedTask
	Finalize     func(context.Context, string, *Session, []CreatedTask, []FailedTask) (*Result, error)
}

func NewBatchTaskExecuteService[Session any, Candidate any, CreatedTask any, FailedTask any, Result any](
	config BatchTaskExecuteServiceConfig[Session, Candidate, CreatedTask, FailedTask, Result],
) *BatchTaskExecuteService[Session, Candidate, CreatedTask, FailedTask, Result] {
	return &BatchTaskExecuteService[Session, Candidate, CreatedTask, FailedTask, Result]{
		loadSession:  config.LoadSession,
		loadItems:    config.LoadItems,
		findExisting: config.FindExisting,
		createTask:   config.CreateTask,
		buildFailed:  config.BuildFailed,
		finalize:     config.Finalize,
	}
}

func (s *BatchTaskExecuteService[Session, Candidate, CreatedTask, FailedTask, Result]) Execute(
	ctx context.Context,
	batchID string,
	designIDs []string,
) (*Result, error) {
	if s == nil || s.loadSession == nil || s.loadItems == nil || s.createTask == nil || s.buildFailed == nil || s.finalize == nil {
		return nil, fmt.Errorf("studio batch task execute service is not configured")
	}
	normalizedBatchID := strings.TrimSpace(batchID)
	session, err := s.loadSession(ctx, normalizedBatchID)
	if err != nil {
		return nil, err
	}
	items, err := s.loadItems(ctx, normalizedBatchID, append([]string(nil), designIDs...))
	if err != nil {
		return nil, err
	}
	createdTasks := make([]CreatedTask, 0, len(items))
	failedTasks := make([]FailedTask, 0)
	for _, item := range items {
		if s.findExisting != nil {
			if existing, ok := s.findExisting(ctx, session, item); ok {
				createdTasks = append(createdTasks, existing)
				continue
			}
		}
		createdTask, createErr := s.createTask(ctx, item)
		if createErr != nil {
			failedTasks = append(failedTasks, s.buildFailed(item, createErr))
			continue
		}
		createdTasks = append(createdTasks, createdTask)
	}
	return s.finalize(ctx, normalizedBatchID, session, createdTasks, failedTasks)
}

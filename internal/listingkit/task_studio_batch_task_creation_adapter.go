package listingkit

import (
	"context"
	"fmt"
	"strings"

	studiodomain "task-processor/internal/listing/studio"
)

type listingStudioBatchTaskCreationRunner = studiodomain.BatchTaskCreationService[
	SheinStudioSession,
	StudioBatchRecord,
	CreateStudioBatchTasksResult,
	SheinStudioCreatedTask,
	SheinStudioFailedTask,
]

func newListingStudioBatchTaskCreationService(s *taskStudioBatchService) *listingStudioBatchTaskCreationRunner {
	return studiodomain.NewBatchTaskCreationService(studiodomain.BatchTaskCreationServiceConfig[
		SheinStudioSession,
		StudioBatchRecord,
		CreateStudioBatchTasksResult,
		SheinStudioCreatedTask,
		SheinStudioFailedTask,
	]{
		PrepareState: func(ctx context.Context, batchID string, designIDs []string) (studiodomain.BatchTaskPrepareState[SheinStudioSession, StudioBatchRecord], error) {
			if s == nil || s.repo == nil {
				return studiodomain.BatchTaskPrepareState[SheinStudioSession, StudioBatchRecord]{}, fmt.Errorf("studio batch repository is not configured")
			}
			state, err := s.buildStudioBatchTaskState(ctx, batchID, designIDs)
			if err != nil {
				return studiodomain.BatchTaskPrepareState[SheinStudioSession, StudioBatchRecord]{}, err
			}
			_, session, _, err := s.prepareStudioBatchTaskCreation(ctx, batchID, &CreateStudioBatchTasksRequest{
				DesignIDs: append([]string(nil), designIDs...),
			})
			if err != nil {
				return studiodomain.BatchTaskPrepareState[SheinStudioSession, StudioBatchRecord]{}, err
			}
			if session == nil {
				session = &SheinStudioSession{
					ID:                   strings.TrimSpace(batchID),
					PendingTaskDesignIDs: append(SheinStudioStringList(nil), state.DesignIDs...),
				}
			}
			return studiodomain.BatchTaskPrepareState[SheinStudioSession, StudioBatchRecord]{
				Session:   session,
				Batch:     state.Batch,
				DesignIDs: state.DesignIDs,
			}, nil
		},
		PrepareTaskCreation: func(ctx context.Context, batchID string, state studiodomain.BatchTaskPrepareState[SheinStudioSession, StudioBatchRecord]) (*CreateStudioBatchTasksResult, error) {
			s.ensureTaskPrepareRunner()
			if s.taskPrepareRunner == nil {
				return nil, fmt.Errorf("studio batch task prepare runner is not configured")
			}
			return s.taskPrepareRunner.PrepareTaskCreation(ctx, batchID, state)
		},
		LoadSession: func(ctx context.Context, batchID string) (*SheinStudioSession, error) {
			if s == nil {
				return nil, nil
			}
			session, err := s.loadStudioBatchTaskSession(ctx, batchID)
			if err != nil && err != ErrStudioSessionNotFound {
				return nil, err
			}
			return session, nil
		},
		PendingDesignIDs: func(session *SheinStudioSession) []string {
			if session == nil {
				return nil
			}
			return normalizeStudioBatchDesignIDs([]string(session.PendingTaskDesignIDs))
		},
		LoadResult: func(ctx context.Context, batchID string) (*CreateStudioBatchTasksResult, error) {
			if s == nil {
				return nil, fmt.Errorf("studio batch task creation service is not configured")
			}
			return s.loadStudioBatchTaskPreparationResult(ctx, batchID)
		},
		CreateTasks: func(ctx context.Context, batchID string, designIDs []string) (*CreateStudioBatchTasksResult, error) {
			if s == nil {
				return nil, fmt.Errorf("studio batch task creation service is not configured")
			}
			return s.CreateStudioBatchTasks(ctx, batchID, &CreateStudioBatchTasksRequest{
				DesignIDs: append([]string(nil), designIDs...),
			})
		},
		LoadBatch: func(ctx context.Context, batchID string) (*StudioBatchRecord, error) {
			if s == nil || s.repo == nil {
				return nil, fmt.Errorf("studio batch repository is not configured")
			}
			return s.repo.GetStudioBatch(ctx, batchID)
		},
		FinalizeTaskCreation: func(ctx context.Context, batchID string, state studiodomain.BatchTaskResumeFinalizeState[SheinStudioSession, StudioBatchRecord, SheinStudioCreatedTask, SheinStudioFailedTask]) (*CreateStudioBatchTasksResult, error) {
			if s == nil {
				return nil, fmt.Errorf("studio batch task creation service is not configured")
			}
			return s.finalizeStudioBatchTaskCreation(ctx, batchID, state.Session, state.Batch, &CreateStudioBatchTasksResult{
				CreatedTasks: append([]SheinStudioCreatedTask(nil), state.CreatedTasks...),
				FailedTasks:  append([]SheinStudioFailedTask(nil), state.FailedTasks...),
			})
		},
		CreatedTasks: func(result *CreateStudioBatchTasksResult) []SheinStudioCreatedTask {
			if result == nil {
				return nil
			}
			return append([]SheinStudioCreatedTask(nil), result.CreatedTasks...)
		},
		FailedTasks: func(result *CreateStudioBatchTasksResult) []SheinStudioFailedTask {
			if result == nil {
				return nil
			}
			return append([]SheinStudioFailedTask(nil), result.FailedTasks...)
		},
	})
}

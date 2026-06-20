package listingkit

import (
	"context"
	"fmt"
	"strings"

	studiodomain "task-processor/internal/listing/studio"
)

type listingStudioBatchTaskExecuteCandidate struct {
	state StudioBatchTaskState
}

type listingStudioBatchTaskExecuteRunner = studiodomain.BatchTaskExecuteService[
	SheinStudioSession,
	listingStudioBatchTaskExecuteCandidate,
	SheinStudioCreatedTask,
	SheinStudioFailedTask,
	CreateStudioBatchTasksResult,
]

func newListingStudioBatchTaskExecuteService(s *taskStudioBatchService) *listingStudioBatchTaskExecuteRunner {
	return studiodomain.NewBatchTaskExecuteService(studiodomain.BatchTaskExecuteServiceConfig[
		SheinStudioSession,
		listingStudioBatchTaskExecuteCandidate,
		SheinStudioCreatedTask,
		SheinStudioFailedTask,
		CreateStudioBatchTasksResult,
	]{
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
		LoadItems: func(ctx context.Context, batchID string, designIDs []string) ([]listingStudioBatchTaskExecuteCandidate, error) {
			if s == nil || s.repo == nil {
				return nil, fmt.Errorf("studio batch repository is not configured")
			}
			state, ok := loadStudioBatchTaskStateFromContext(ctx, batchID)
			if !ok {
				var err error
				state, err = s.prepareStudioBatchTaskExecuteCandidates(ctx, batchID, designIDs)
				if err != nil {
					return nil, err
				}
			}
			items := make([]listingStudioBatchTaskExecuteCandidate, 0, len(state.Candidates))
			for _, candidate := range state.Candidates {
				items = append(items, listingStudioBatchTaskExecuteCandidate{
					state: StudioBatchTaskState{
						Session:       state.Session,
						Batch:         state.Batch,
						DesignIDs:     append([]string(nil), state.DesignIDs...),
						Candidates:    []studioBatchTaskCandidate{candidate},
						RejectedTasks: append([]SheinStudioRejectedTask(nil), state.RejectedTasks...),
					},
				})
			}
			return items, nil
		},
		FindExisting: func(ctx context.Context, session *SheinStudioSession, candidate listingStudioBatchTaskExecuteCandidate) (SheinStudioCreatedTask, bool) {
			if s == nil || session == nil {
				return SheinStudioCreatedTask{}, false
			}
			return s.findExistingStudioBatchTask(ctx, session.CreatedTasks, candidate.state.Candidates[0])
		},
		CreateTask: func(ctx context.Context, candidate listingStudioBatchTaskExecuteCandidate) (SheinStudioCreatedTask, error) {
			if s == nil || s.createGenerateTask == nil {
				return SheinStudioCreatedTask{}, fmt.Errorf("listing task creator is not configured")
			}
			taskCandidate := candidate.state.Candidates[0]
			task, err := s.createGenerateTask(
				ctx,
				buildStudioBatchTaskGenerateRequest(
					candidate.state.Session,
					candidate.state.Batch,
					taskCandidate,
					taskCandidate.Design,
				),
			)
			if err != nil {
				return SheinStudioCreatedTask{}, err
			}
			return SheinStudioCreatedTask{
				ID:       task.ID,
				Title:    taskCandidate.Title,
				DesignID: taskCandidate.Design.ID,
			}, nil
		},
		BuildFailed: func(candidate listingStudioBatchTaskExecuteCandidate, err error) SheinStudioFailedTask {
			taskCandidate := candidate.state.Candidates[0]
			return SheinStudioFailedTask{
				DesignID: taskCandidate.Design.ID,
				Title:    taskCandidate.Title,
				Message:  err.Error(),
			}
		},
		Finalize: func(ctx context.Context, batchID string, session *SheinStudioSession, created []SheinStudioCreatedTask, failed []SheinStudioFailedTask) (*CreateStudioBatchTasksResult, error) {
			if s == nil || s.repo == nil {
				return nil, fmt.Errorf("studio batch repository is not configured")
			}
			state, ok := loadStudioBatchTaskStateFromContext(ctx, batchID)
			if !ok {
				var err error
				state, err = s.buildStudioBatchTaskState(ctx, batchID, designIDsFromCreatedAndFailedTasks(created, failed))
				if err != nil {
					return nil, err
				}
			}
			return s.completeStudioBatchTaskExecution(ctx, batchID, session, state.Batch, created, state.RejectedTasks, failed)
		},
	})
}

func designIDsFromCreatedAndFailedTasks(created []SheinStudioCreatedTask, failed []SheinStudioFailedTask) []string {
	seen := make(map[string]struct{}, len(created)+len(failed))
	designIDs := make([]string, 0, len(created)+len(failed))
	appendDesignID := func(raw string) {
		designID := strings.TrimSpace(raw)
		if designID == "" {
			return
		}
		if _, ok := seen[designID]; ok {
			return
		}
		seen[designID] = struct{}{}
		designIDs = append(designIDs, designID)
	}
	for _, task := range created {
		appendDesignID(task.DesignID)
	}
	for _, task := range failed {
		appendDesignID(task.DesignID)
	}
	return designIDs
}

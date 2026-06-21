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
						Session:              state.Session,
						Batch:                state.Batch,
						DesignIDs:            append([]string(nil), state.DesignIDs...),
						AllApprovedDesignIDs: append([]string(nil), state.AllApprovedDesignIDs...),
						Candidates:           []studioBatchTaskCandidate{candidate},
						RejectedTasks:        append([]SheinStudioRejectedTask(nil), state.RejectedTasks...),
						FailedTasks:          append([]SheinStudioFailedTask(nil), state.FailedTasks...),
					},
				})
			}
			return items, nil
		},
		FindExisting: func(ctx context.Context, session *SheinStudioSession, candidate listingStudioBatchTaskExecuteCandidate) (SheinStudioCreatedTask, bool) {
			if s == nil || len(candidate.state.Candidates) == 0 {
				return SheinStudioCreatedTask{}, false
			}
			var recorded SheinStudioCreatedTaskList
			if session != nil {
				recorded = session.CreatedTasks
			}
			return s.findExistingStudioBatchTask(ctx, recorded, candidate.state.Candidates[0])
		},
		CreateTask: func(ctx context.Context, candidate listingStudioBatchTaskExecuteCandidate) (SheinStudioCreatedTask, error) {
			if s == nil || s.createGenerateTask == nil {
				return SheinStudioCreatedTask{}, fmt.Errorf("listing task creator is not configured")
			}
			taskCandidate := candidate.state.Candidates[0]
			if err := s.reserveStudioBatchTaskCandidate(ctx, taskCandidate); err != nil {
				return SheinStudioCreatedTask{}, err
			}
			claimed, err := s.claimStudioBatchTaskCandidate(ctx, taskCandidate)
			if err != nil {
				return SheinStudioCreatedTask{}, err
			}
			if !claimed {
				if existing, ok := s.findDurableStudioBatchTask(ctx, taskCandidate); ok {
					return existing, nil
				}
				return SheinStudioCreatedTask{}, fmt.Errorf("studio batch task candidate is already owned")
			}
			if candidate.state.Session != nil {
				if existing, ok, err := s.findLegacyStudioBatchTask(ctx, candidate.state.Session.CreatedTasks, taskCandidate); err != nil {
					return SheinStudioCreatedTask{}, err
				} else if ok {
					return existing, nil
				}
			}
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
				taskID := ""
				if task != nil {
					taskID = task.ID
				}
				_ = s.persistStudioBatchTaskLink(ctx, taskCandidate, taskID, studioBatchTaskLinkStatusFailed, "task_create_failed", err.Error())
				return SheinStudioCreatedTask{}, err
			}
			created := SheinStudioCreatedTask{
				ID:                       task.ID,
				Title:                    taskCandidate.Title,
				DesignID:                 taskCandidate.Design.ID,
				ItemID:                   taskCandidate.Item.ID,
				SelectionID:              taskCandidate.SelectionID,
				CompatibilityFingerprint: taskCandidate.CompatibilityFingerprint,
				Status:                   studioBatchCreatedTaskStatus,
			}
			if err := s.persistStudioBatchTaskLink(ctx, taskCandidate, task.ID, studioBatchTaskLinkStatusCreated, "", ""); err != nil {
				return SheinStudioCreatedTask{}, err
			}
			return created, nil
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
			allFailed := append([]SheinStudioFailedTask(nil), state.FailedTasks...)
			allFailed = append(allFailed, failed...)
			shouldMarkTasksCreated := len(created) == len(state.Candidates) &&
				len(state.RejectedTasks) == 0 &&
				len(allFailed) == 0 &&
				equalNormalizedStudioBatchDesignIDSets(state.DesignIDs, state.AllApprovedDesignIDs)
			return s.completeStudioBatchTaskExecution(ctx, batchID, session, state.Batch, created, state.RejectedTasks, allFailed, shouldMarkTasksCreated)
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

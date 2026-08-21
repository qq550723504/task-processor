package listingkit

import (
	"context"
	"errors"
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
			latestDesigns, err := s.repo.ListStudioMaterializedDesignsByIDs(ctx, taskCandidate.Design.BatchID, []string{taskCandidate.Design.ID})
			if err != nil {
				return SheinStudioCreatedTask{}, err
			}
			if len(latestDesigns) != 1 {
				return SheinStudioCreatedTask{}, fmt.Errorf("studio design %s is no longer available", taskCandidate.Design.ID)
			}
			if latestDesigns[0].BackgroundRemovalStatus == StudioBackgroundRemovalStatusPending {
				return SheinStudioCreatedTask{}, fmt.Errorf("studio design %s background removal is still in progress", taskCandidate.Design.ID)
			}
			taskCandidate.Design = latestDesigns[0]
			if err := s.reserveStudioBatchTaskCandidate(ctx, &taskCandidate); err != nil {
				return SheinStudioCreatedTask{}, err
			}
			claimed, err := s.claimStudioBatchTaskCandidate(ctx, &taskCandidate)
			if err != nil {
				return SheinStudioCreatedTask{}, err
			}
			if !claimed {
				if existing, ok := s.findDurableStudioBatchTask(ctx, taskCandidate); ok {
					return markStudioBatchReusedTask(existing), nil
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
			generateRequest := buildStudioBatchTaskGenerateRequest(
				candidate.state.Session,
				candidate.state.Batch,
				taskCandidate,
				taskCandidate.Design,
			)
			if generateRequest == nil {
				return SheinStudioCreatedTask{}, fmt.Errorf("studio batch task request is not configured")
			}
			dispatchCtx, dispatchHeartbeatStop := s.startStudioBatchTaskLinkHeartbeatContext(ctx, taskCandidate, studioBatchTaskLinkHeartbeatInterval)
			if strings.TrimSpace(taskCandidate.ClaimToken) != "" {
				dispatchCtx = withTaskDispatchCancellation(withStudioBatchTaskLinkHeartbeat(dispatchCtx))
			}
			if err := s.attachStudioBatchProductImages(dispatchCtx, generateRequest, candidate.state.Session, candidate.state.Batch, taskCandidate, taskCandidate.Design); err != nil {
				reasonCode := "product_image_generation_failed"
				if s.generateProductImages == nil {
					reasonCode = "product_image_generation_unavailable"
				} else if strings.Contains(err.Error(), "returned no images") {
					reasonCode = "product_image_generation_empty"
				}
				persistErr := s.persistStudioBatchTaskLink(dispatchCtx, taskCandidate, "", studioBatchTaskLinkStatusFailed, studioBatchTaskLinkSourceBatchCreated, reasonCode, err.Error())
				_ = dispatchHeartbeatStop()
				if persistErr != nil {
					return SheinStudioCreatedTask{}, persistErr
				}
				return SheinStudioCreatedTask{}, err
			}
			if err := s.revalidateStudioBatchTaskLinkLease(dispatchCtx, taskCandidate); err != nil {
				_ = dispatchHeartbeatStop()
				return SheinStudioCreatedTask{}, err
			}
			task, err := s.createGenerateTask(
				dispatchCtx,
				generateRequest,
			)
			if err != nil {
				taskID := ""
				if task != nil {
					taskID = task.ID
				}
				if errors.Is(err, context.Canceled) {
					// createGenerateTask may have inserted a queued row before
					// lease cancellation reached dispatch. Do not retain its ID:
					// linking it as a failed batch task would make the never-
					// dispatched row eligible for durable-task reuse on retry.
					taskID = ""
				}
				persistErr := s.persistStudioBatchTaskLink(dispatchCtx, taskCandidate, taskID, studioBatchTaskLinkStatusFailed, studioBatchTaskLinkSourceBatchCreated, "task_create_failed", err.Error())
				_ = dispatchHeartbeatStop()
				if persistErr != nil {
					return SheinStudioCreatedTask{}, persistErr
				}
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
				Source:                   studioBatchTaskLinkSourceBatchCreated,
			}
			if err := s.persistStudioBatchTaskLink(dispatchCtx, taskCandidate, task.ID, studioBatchTaskLinkStatusCreated, studioBatchTaskLinkSourceBatchCreated, "", ""); err != nil {
				_ = dispatchHeartbeatStop()
				return SheinStudioCreatedTask{}, err
			}
			// Settle only after both the ListingKit task and durable created link
			// exist. Keep the ledger write alive if the caller/lease context ends
			// immediately after the terminal commit.
			s.settleStudioBatchProductImageUsage(context.WithoutCancel(dispatchCtx), candidate.state.Batch, taskCandidate)
			if err := dispatchHeartbeatStop(); err != nil {
				return SheinStudioCreatedTask{}, err
			}
			return created, nil
		},
		BuildFailed: func(candidate listingStudioBatchTaskExecuteCandidate, err error) SheinStudioFailedTask {
			taskCandidate := candidate.state.Candidates[0]
			return SheinStudioFailedTask{
				DesignID: taskCandidate.Design.ID,
				Title:    taskCandidate.Title,
				Source:   studioBatchTaskLinkSourceBatchCreated,
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

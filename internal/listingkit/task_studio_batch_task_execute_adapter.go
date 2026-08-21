package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	studiodomain "task-processor/internal/listing/studio"
	"task-processor/internal/listingkit/core"
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
			batchCandidate := candidate.state.Candidates[0]
			var recorded SheinStudioCreatedTaskList
			if session != nil {
				recorded = session.CreatedTasks
			}
			if existing, ok := s.findDurableStudioBatchTask(ctx, batchCandidate); ok {
				reusedCandidate := batchCandidate
				if s.batchTaskLinkRepo != nil {
					candidateKeys := []string{batchCandidate.CandidateKey}
					if historicalKey := strings.TrimSpace(batchCandidate.HistoricalCandidateKey); historicalKey != "" && historicalKey != strings.TrimSpace(batchCandidate.CandidateKey) {
						candidateKeys = append(candidateKeys, historicalKey)
					}
					for _, candidateKey := range candidateKeys {
						if existingLink, linkErr := s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, candidateKey); linkErr == nil && existingLink != nil {
							reusedCandidate.ClaimToken = existingLink.ClaimToken
							break
						}
					}
				}
				if err := s.settleStudioBatchProductImageUsage(context.WithoutCancel(ctx), candidate.state.Batch, reusedCandidate); err != nil {
					return SheinStudioCreatedTask{}, false
				}
				return markStudioBatchReusedTask(existing), true
			}
			if session == nil || len(recorded) == 0 {
				return SheinStudioCreatedTask{}, false
			}
			existing, ok, err := s.findLegacyStudioBatchTask(ctx, recorded, batchCandidate)
			if err != nil || !ok {
				return SheinStudioCreatedTask{}, false
			}
			return markStudioBatchReusedTask(existing), true
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
			if err := s.releasePendingStudioBatchProductImageUsage(ctx, candidate.state.Batch, taskCandidate); err != nil {
				return SheinStudioCreatedTask{}, fmt.Errorf("release pending product image usage reservation: %w", err)
			}
			if err := s.releaseFailedStudioBatchProductImageReservationBeforeReclaim(ctx, candidate.state.Batch, taskCandidate); err != nil {
				return SheinStudioCreatedTask{}, fmt.Errorf("release failed product image usage reservation before reclaim: %w", err)
			}
			if err := s.reserveStudioBatchTaskCandidate(ctx, &taskCandidate); err != nil {
				return SheinStudioCreatedTask{}, err
			}
			claimed, previousClaimToken, err := s.claimStudioBatchTaskCandidate(ctx, &taskCandidate)
			if err != nil {
				return SheinStudioCreatedTask{}, err
			}
			if claimed && strings.TrimSpace(previousClaimToken) != "" {
				previousCandidate := taskCandidate
				previousCandidate.ClaimToken = previousClaimToken
				if err := s.releaseStudioBatchProductImageUsage(context.WithoutCancel(ctx), candidate.state.Batch, previousCandidate, "stale_reclaimed"); err != nil {
					persistErr := s.persistPendingStudioBatchProductImageUsageRelease(context.WithoutCancel(ctx), taskCandidate, previousClaimToken)
					if persistErr != nil {
						return SheinStudioCreatedTask{}, errors.Join(fmt.Errorf("release reclaimed product image usage: %w", err), fmt.Errorf("persist pending product image usage release: %w", persistErr))
					}
					return SheinStudioCreatedTask{}, fmt.Errorf("release reclaimed product image usage reservation: %w", err)
				}
				if err := s.clearPendingStudioBatchProductImageUsageRelease(context.WithoutCancel(ctx), taskCandidate); err != nil {
					return SheinStudioCreatedTask{}, fmt.Errorf("clear reclaimed product image usage release marker: %w", err)
				}
			}
			if !claimed {
				if existing, ok := s.findDurableStudioBatchTask(ctx, taskCandidate); ok {
					reusedCandidate := taskCandidate
					if s.batchTaskLinkRepo != nil {
						if existingLink, linkErr := s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, taskCandidate.CandidateKey); linkErr == nil && existingLink != nil {
							reusedCandidate.ClaimToken = existingLink.ClaimToken
						}
					}
					if err := s.settleStudioBatchProductImageUsage(context.WithoutCancel(ctx), candidate.state.Batch, reusedCandidate); err != nil {
						return SheinStudioCreatedTask{}, err
					}
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
				releaseErr := s.releaseStudioBatchProductImageUsage(context.WithoutCancel(dispatchCtx), candidate.state.Batch, taskCandidate, reasonCode)
				_ = dispatchHeartbeatStop()
				if releaseErr != nil {
					err = errors.Join(err, fmt.Errorf("release product image usage: %w", releaseErr))
				}
				if persistErr != nil {
					return SheinStudioCreatedTask{}, errors.Join(persistErr, err)
				}
				return SheinStudioCreatedTask{}, err
			}
			if err := s.revalidateStudioBatchTaskDesign(dispatchCtx, taskCandidate); err != nil {
				persistErr := s.persistStudioBatchTaskLink(dispatchCtx, taskCandidate, "", studioBatchTaskLinkStatusFailed, studioBatchTaskLinkSourceBatchCreated, "design_changed_during_generation", err.Error())
				releaseErr := s.releaseStudioBatchProductImageUsage(context.WithoutCancel(dispatchCtx), candidate.state.Batch, taskCandidate, "design_changed_during_generation")
				_ = dispatchHeartbeatStop()
				if releaseErr != nil {
					err = errors.Join(err, fmt.Errorf("release product image usage: %w", releaseErr))
				}
				if persistErr != nil {
					return SheinStudioCreatedTask{}, errors.Join(persistErr, err)
				}
				return SheinStudioCreatedTask{}, err
			}
			if err := s.revalidateStudioBatchTaskLinkLease(dispatchCtx, taskCandidate); err != nil {
				releaseErr := s.releaseStudioBatchProductImageUsage(context.WithoutCancel(dispatchCtx), candidate.state.Batch, taskCandidate, "lease_lost")
				_ = dispatchHeartbeatStop()
				if releaseErr != nil {
					return SheinStudioCreatedTask{}, errors.Join(err, fmt.Errorf("release product image usage: %w", releaseErr))
				}
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
					terminalTask := task
					if s.getTask != nil && taskID != "" {
						if loaded, loadErr := s.getTask(context.WithoutCancel(dispatchCtx), taskID); loadErr == nil && loaded != nil {
							terminalTask = loaded
						}
					}
					if terminalTask != nil && isTerminalStudioBatchGeneratedTask(terminalTask) {
						durableCtx := context.WithoutCancel(dispatchCtx)
						if persistErr := s.persistStudioBatchTaskLink(durableCtx, taskCandidate, terminalTask.ID, studioBatchTaskLinkStatusCreated, studioBatchTaskLinkSourceBatchCreated, "", ""); persistErr == nil {
							created := SheinStudioCreatedTask{ID: terminalTask.ID, Title: taskCandidate.Title, DesignID: taskCandidate.Design.ID, ItemID: taskCandidate.Item.ID, SelectionID: taskCandidate.SelectionID, CompatibilityFingerprint: taskCandidate.CompatibilityFingerprint, Status: studioBatchCreatedTaskStatus, Source: studioBatchTaskLinkSourceBatchCreated}
							// The durable task/link already exist. If settlement is
							// temporarily unavailable, return the terminal task and let
							// the durable reuse path retry settlement; never release its
							// reservation as if task creation had failed.
							_ = s.settleStudioBatchProductImageUsage(durableCtx, candidate.state.Batch, taskCandidate)
							_ = dispatchHeartbeatStop()
							return created, nil
						}
					}
					// A queued or processing row is not safe to reuse after cancellation.
					taskID = ""
				}
				persistErr := s.persistStudioBatchTaskLink(dispatchCtx, taskCandidate, taskID, studioBatchTaskLinkStatusFailed, studioBatchTaskLinkSourceBatchCreated, "task_create_failed", err.Error())
				releaseErr := s.releaseStudioBatchProductImageUsage(context.WithoutCancel(dispatchCtx), candidate.state.Batch, taskCandidate, "task_create_failed")
				_ = dispatchHeartbeatStop()
				if releaseErr != nil {
					err = errors.Join(err, fmt.Errorf("release product image usage: %w", releaseErr))
				}
				if persistErr != nil {
					return SheinStudioCreatedTask{}, errors.Join(persistErr, err)
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
			if err := s.settleStudioBatchProductImageUsage(context.WithoutCancel(dispatchCtx), candidate.state.Batch, taskCandidate); err != nil {
				_ = dispatchHeartbeatStop()
				return SheinStudioCreatedTask{}, fmt.Errorf("settle product image usage: %w", err)
			}
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

// revalidateStudioBatchTaskDesign closes the race between reading the approved
// design and completing the potentially long product-image generation call.
// A background-removal retry may replace the image while generation is in
// flight; generated output for that superseded design must never be linked to
// a newly created ListingKit task.
func (s *taskStudioBatchService) revalidateStudioBatchTaskDesign(ctx context.Context, candidate studioBatchTaskCandidate) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("studio batch repository is not configured")
	}
	designs, err := s.repo.ListStudioMaterializedDesignsByIDs(ctx, candidate.Design.BatchID, []string{candidate.Design.ID})
	if err != nil {
		return fmt.Errorf("revalidate studio design %s: %w", strings.TrimSpace(candidate.Design.ID), err)
	}
	if len(designs) != 1 {
		return fmt.Errorf("studio design %s is no longer available", strings.TrimSpace(candidate.Design.ID))
	}
	latest := designs[0]
	if strings.TrimSpace(latest.ImageURL) != strings.TrimSpace(candidate.Design.ImageURL) ||
		strings.TrimSpace(latest.OriginalImageURL) != strings.TrimSpace(candidate.Design.OriginalImageURL) ||
		latest.BackgroundRemovalStatus != candidate.Design.BackgroundRemovalStatus ||
		(!candidate.Design.UpdatedAt.IsZero() && !latest.UpdatedAt.Equal(candidate.Design.UpdatedAt)) {
		return fmt.Errorf("studio design %s changed while product images were generating", strings.TrimSpace(candidate.Design.ID))
	}
	return nil
}

func isTerminalStudioBatchGeneratedTask(task *Task) bool {
	if task == nil {
		return false
	}
	return task.Status == core.TaskStatusCompleted || task.Status == core.TaskStatusNeedsReview
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

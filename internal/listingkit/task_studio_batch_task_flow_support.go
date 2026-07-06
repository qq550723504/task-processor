package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type studioBatchPartialTaskCreationContextKey struct{}

func withStudioBatchPartialTaskCreationAllowed(ctx context.Context) context.Context {
	return context.WithValue(ctx, studioBatchPartialTaskCreationContextKey{}, true)
}

func isStudioBatchPartialTaskCreationAllowed(ctx context.Context) bool {
	allowed, _ := ctx.Value(studioBatchPartialTaskCreationContextKey{}).(bool)
	return allowed
}

func (s *taskStudioBatchService) resumeStudioBatchTaskCreation(ctx context.Context, batchID string) (*CreateStudioBatchTasksResult, error) {
	s.ensureTaskResumeRunner()
	if s.taskResumeRunner == nil {
		return nil, fmt.Errorf("studio batch task resume runner is not configured")
	}
	s.ensureTaskCreationRunner()
	if s.taskCreationRunner == nil {
		return nil, fmt.Errorf("studio batch task creation service is not configured")
	}
	return s.taskCreationRunner.ResumeTaskCreation(ctx, batchID)
}

func (s *taskStudioBatchService) loadStudioBatchTaskPreparationResult(ctx context.Context, batchID string) (*CreateStudioBatchTasksResult, error) {
	detail, err := s.GetStudioBatchDetail(ctx, batchID)
	if err != nil {
		return nil, err
	}
	return &CreateStudioBatchTasksResult{
		Batch:        detail.Batch,
		Items:        detail.Items,
		CreatedTasks: detail.CreatedTasks,
		FailedTasks:  detail.FailedTasks,
	}, nil
}

func (s *taskStudioBatchService) reserveStudioBatchTaskCandidate(ctx context.Context, candidate studioBatchTaskCandidate) error {
	if s == nil || s.batchTaskLinkRepo == nil {
		return nil
	}
	now := s.currentTime().UTC()
	link := &StudioBatchTaskLinkRecord{
		ID:                       buildStudioBatchTaskLinkID(candidate),
		BatchID:                  strings.TrimSpace(candidate.Design.BatchID),
		ItemID:                   strings.TrimSpace(candidate.Item.ID),
		DesignID:                 strings.TrimSpace(candidate.Design.ID),
		SelectionID:              strings.TrimSpace(candidate.SelectionID),
		CompatibilityFingerprint: strings.TrimSpace(candidate.CompatibilityFingerprint),
		SheinStoreID:             candidate.SheinStoreID,
		CandidateKey:             strings.TrimSpace(candidate.CandidateKey),
		Status:                   studioBatchTaskLinkStatusReserved,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if link.BatchID == "" {
		link.BatchID = strings.TrimSpace(candidate.Item.BatchID)
	}
	if err := s.batchTaskLinkRepo.CreateStudioBatchTaskLink(ctx, link); err != nil {
		if _, getErr := s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey); getErr == nil {
			return nil
		}
		return err
	}
	return nil
}

func (s *taskStudioBatchService) claimStudioBatchTaskCandidate(ctx context.Context, candidate studioBatchTaskCandidate) (bool, error) {
	if s == nil || s.batchTaskLinkRepo == nil {
		return true, nil
	}
	now := s.currentTime().UTC()
	existing, existingErr := s.batchTaskLinkRepo.GetStudioBatchTaskLinkByCandidateKey(ctx, candidate.CandidateKey)
	if existingErr != nil && !errors.Is(existingErr, gorm.ErrRecordNotFound) {
		return false, existingErr
	}
	if existingErr == nil && existing != nil {
		if existing.Status == studioBatchTaskLinkStatusFailed && strings.TrimSpace(existing.ListingKitTaskID) != "" {
			return false, nil
		}
		if s.studioBatchTaskLinkIsStale(existing) {
			if _, claimed, err := s.batchTaskLinkRepo.ClaimStudioBatchTaskCandidateUpdatedAt(ctx, candidate.CandidateKey, studioBatchTaskLinkStatusCreating, existing.UpdatedAt, studioBatchTaskLinkStatusCreating, now); err != nil {
				return false, err
			} else if claimed {
				return true, nil
			}
		}
	}
	if _, claimed, err := s.batchTaskLinkRepo.ClaimStudioBatchTaskCandidate(ctx, candidate.CandidateKey, studioBatchTaskLinkStatusReserved, studioBatchTaskLinkStatusCreating, now); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return false, err
		}
	} else if claimed {
		return true, nil
	}
	if _, claimed, err := s.batchTaskLinkRepo.ClaimStudioBatchTaskCandidate(ctx, candidate.CandidateKey, studioBatchTaskLinkStatusFailed, studioBatchTaskLinkStatusCreating, now); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return false, err
		}
	} else if claimed {
		return true, nil
	}
	return false, nil
}

func (s *taskStudioBatchService) completeStudioBatchTaskExecution(
	ctx context.Context,
	batchID string,
	session *SheinStudioSession,
	batch *StudioBatchRecord,
	createdTasks []SheinStudioCreatedTask,
	rejectedTasks []SheinStudioRejectedTask,
	failedTasks []SheinStudioFailedTask,
	markTasksCreated bool,
) (*CreateStudioBatchTasksResult, error) {
	newlyCreatedTasks, reusedTasks, ownedTasks := splitStudioBatchCreatedAndReusedTasks(createdTasks)
	if sessionUpdater, ok := s.studioSessionRepo.(interface {
		UpdateSession(context.Context, *SheinStudioSession) error
	}); ok && session != nil {
		session.CreatedTasks = mergeStudioCreatedTasks(session.CreatedTasks, ownedTasks)
		session.CreatedTaskIDs = buildCreatedTaskIDs(session.CreatedTasks)
		session.FailedTasks = append(SheinStudioFailedTaskList(nil), failedTasks...)
		session.PendingTaskDesignIDs = nil
		if markTasksCreated && len(ownedTasks) > 0 {
			session.Status = SheinStudioSessionStatusTasksCreated
		} else if session.Status == SheinStudioSessionStatusTasksCreating {
			session.Status = SheinStudioSessionStatusReviewing
		}
		session.UpdatedAt = s.currentTime().UTC()
		if err := sessionUpdater.UpdateSession(ctx, session); err != nil {
			return nil, err
		}
	}
	if batch != nil {
		if markTasksCreated && len(ownedTasks) > 0 {
			batch.Status = StudioBatchStatusTasksCreated
		} else if batch.Status == StudioBatchStatusTasksCreating {
			if graph, err := s.repo.GetStudioBatchDetail(ctx, batchID); err == nil && graph != nil {
				batch.Status = resolveProjectedStudioBatchStatus("", graph.Items)
			}
		}
		batch.UpdatedAt = s.currentTime().UTC()
		if err := s.repo.UpdateStudioBatch(ctx, batch); err != nil {
			return nil, err
		}
	}
	detail, err := s.GetStudioBatchDetail(ctx, batchID)
	if err != nil {
		return nil, err
	}
	return &CreateStudioBatchTasksResult{
		Batch:         detail.Batch,
		Items:         detail.Items,
		CreatedTasks:  newlyCreatedTasks,
		ReusedTasks:   reusedTasks,
		RejectedTasks: rejectedTasks,
		FailedTasks:   failedTasks,
	}, nil
}

func splitStudioBatchCreatedAndReusedTasks(tasks []SheinStudioCreatedTask) ([]SheinStudioCreatedTask, []SheinStudioCreatedTask, []SheinStudioCreatedTask) {
	if len(tasks) == 0 {
		return nil, nil, nil
	}
	created := make([]SheinStudioCreatedTask, 0, len(tasks))
	reused := make([]SheinStudioCreatedTask, 0)
	owned := make([]SheinStudioCreatedTask, 0, len(tasks))
	for _, task := range tasks {
		isReused := strings.TrimSpace(task.ReasonCode) == studioBatchReusedTaskReasonCode
		if isReused {
			task.ReasonCode = ""
		}
		owned = append(owned, task)
		if isReused {
			reused = append(reused, task)
			continue
		}
		created = append(created, task)
	}
	return created, reused, owned
}

func (s *taskStudioBatchService) finalizeStudioBatchTaskCreation(
	ctx context.Context,
	batchID string,
	session *SheinStudioSession,
	batch *StudioBatchRecord,
	result *CreateStudioBatchTasksResult,
) (*CreateStudioBatchTasksResult, error) {
	s.ensureTaskResumeRunner()
	if s.taskResumeRunner == nil {
		return nil, fmt.Errorf("studio batch task resume runner is not configured")
	}
	var createdTasks []SheinStudioCreatedTask
	var failedTasks []SheinStudioFailedTask
	if result != nil {
		createdTasks = result.CreatedTasks
		failedTasks = result.FailedTasks
	}
	return s.taskResumeRunner.FinalizeTaskCreation(ctx, batchID, listingStudioBatchTaskResumeState{
		Session:      session,
		Batch:        batch,
		CreatedTasks: createdTasks,
		FailedTasks:  failedTasks,
	})
}

func (s *taskStudioBatchService) prepareStudioBatchTaskExecuteCandidates(
	ctx context.Context,
	batchID string,
	designIDs []string,
) (*StudioBatchTaskState, error) {
	state, err := s.buildStudioBatchTaskState(ctx, batchID, designIDs)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (s *taskStudioBatchService) prepareStudioBatchTaskCreation(
	ctx context.Context,
	batchID string,
	req *CreateStudioBatchTasksRequest,
) ([]string, *SheinStudioSession, *StudioBatchDetailGraph, error) {
	designIDs := normalizeStudioBatchDesignIDs(nil)
	if req != nil {
		designIDs = normalizeStudioBatchDesignIDs(req.DesignIDs)
	}
	if len(designIDs) == 0 {
		return nil, nil, nil, NewStudioBatchActionValidationError("design_ids is required")
	}
	designs, err := s.repo.ListStudioMaterializedDesignsByIDs(ctx, batchID, designIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(designs) != len(designIDs) {
		return nil, nil, nil, gorm.ErrRecordNotFound
	}
	batchDetail, err := s.repo.GetStudioBatchDetail(ctx, batchID)
	if err != nil {
		return nil, nil, nil, err
	}
	if batchDetail == nil || batchDetail.Batch == nil {
		return nil, nil, nil, gorm.ErrRecordNotFound
	}
	session, err := s.loadStudioBatchTaskSession(ctx, batchID)
	if err != nil {
		if !errors.Is(err, ErrStudioSessionNotFound) {
			return nil, nil, nil, err
		}
		session = nil
	}
	allowPartialWhileGenerating := isStudioBatchPartialTaskCreationAllowed(ctx) ||
		(req != nil && req.AllowPartialWhileGenerating)
	if !allowPartialWhileGenerating && session != nil {
		allowPartialWhileGenerating = equalNormalizedStudioBatchDesignIDs(
			[]string(session.PendingTaskDesignIDs),
			designIDs,
		)
	}
	if !allowPartialWhileGenerating && studioBatchTaskCreationHasInFlightGeneration(batchDetail) {
		return nil, nil, nil, NewStudioBatchActionValidationError("batch is still generating; confirm partial task creation before creating SHEIN tasks")
	}
	for _, design := range designs {
		if design.ReviewStatus != StudioMaterializedDesignReviewStatusApproved {
			return nil, nil, nil, NewStudioBatchActionValidationError(fmt.Sprintf("design %s is not approved", design.ID))
		}
	}
	if err := validateStudioBatchTaskCreationDesignReadiness(designs, batchDetail); err != nil {
		return nil, nil, nil, err
	}
	return designIDs, session, batchDetail, nil
}

func validateStudioBatchTaskCreationDesignReadiness(
	designs []StudioMaterializedDesignRecord,
	detail *StudioBatchDetailGraph,
) error {
	if !studioBatchTaskCreationRequiresReadyItems(detail) {
		return nil
	}
	itemsByID := make(map[string]StudioBatchItemRecord, len(detail.Items))
	for _, item := range detail.Items {
		if itemID := strings.TrimSpace(item.ID); itemID != "" {
			itemsByID[itemID] = item
		}
	}
	for _, design := range designs {
		itemID := strings.TrimSpace(design.ItemID)
		item, ok := itemsByID[itemID]
		if !ok {
			return NewStudioBatchActionValidationError(fmt.Sprintf("design %s belongs to unknown item %s", design.ID, itemID))
		}
		if item.Status != StudioBatchItemStatusReviewReady {
			return NewStudioBatchActionValidationError(fmt.Sprintf("design %s belongs to item %s with status %s; item is not ready for task creation", design.ID, item.ID, item.Status))
		}
	}
	return nil
}

func studioBatchTaskCreationRequiresReadyItems(detail *StudioBatchDetailGraph) bool {
	if detail == nil || detail.Batch == nil {
		return false
	}
	switch detail.Batch.Status {
	case StudioBatchStatusGenerating,
		StudioBatchStatusPartiallyMaterialized,
		StudioBatchStatusReviewReady,
		StudioBatchStatusPartiallyFailed,
		StudioBatchStatusTasksCreating,
		StudioBatchStatusTasksCreated:
		return true
	default:
		return false
	}
}

func studioBatchTaskCreationHasInFlightGeneration(detail *StudioBatchDetailGraph) bool {
	if detail == nil || detail.Batch == nil {
		return false
	}
	if detail.Batch.Status == StudioBatchStatusGenerating {
		return true
	}
	for _, item := range detail.Items {
		if item.Status == StudioBatchItemStatusGenerating ||
			item.Status == StudioBatchItemStatusAwaitingMaterialization {
			return true
		}
	}
	return false
}

func equalNormalizedStudioBatchDesignIDs(left []string, right []string) bool {
	normalizedLeft := normalizeStudioBatchDesignIDs(left)
	normalizedRight := normalizeStudioBatchDesignIDs(right)
	if len(normalizedLeft) != len(normalizedRight) {
		return false
	}
	for index := range normalizedLeft {
		if normalizedLeft[index] != normalizedRight[index] {
			return false
		}
	}
	return len(normalizedLeft) > 0
}

func equalNormalizedStudioBatchDesignIDSets(left []string, right []string) bool {
	normalizedLeft := normalizeStudioBatchDesignIDs(left)
	normalizedRight := normalizeStudioBatchDesignIDs(right)
	if len(normalizedLeft) == 0 || len(normalizedLeft) != len(normalizedRight) {
		return false
	}
	rightSet := make(map[string]struct{}, len(normalizedRight))
	for _, designID := range normalizedRight {
		rightSet[designID] = struct{}{}
	}
	for _, designID := range normalizedLeft {
		if _, ok := rightSet[designID]; !ok {
			return false
		}
	}
	return true
}

func (s *taskStudioBatchService) loadStudioBatchTaskSession(ctx context.Context, batchID string) (*SheinStudioSession, error) {
	if s.studioSessionRepo == nil {
		return nil, ErrStudioSessionNotFound
	}
	session, err := s.studioSessionRepo.GetSession(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrStudioSessionNotFound
	}
	return session, nil
}

func (s *taskStudioBatchService) loadStudioBatchSessionDesigns(
	ctx context.Context,
	batchID string,
) (map[string]SheinStudioDesign, error) {
	if s == nil || s.studioSessionRepo == nil {
		return map[string]SheinStudioDesign{}, nil
	}
	designSource, ok := s.studioSessionRepo.(interface {
		ListSessionDesigns(context.Context, string) ([]SheinStudioDesign, error)
	})
	if !ok {
		return map[string]SheinStudioDesign{}, nil
	}
	sessionDesigns, err := designSource.ListSessionDesigns(ctx, batchID)
	if err != nil {
		return nil, err
	}
	sessionDesignsByID := make(map[string]SheinStudioDesign, len(sessionDesigns))
	for _, design := range sessionDesigns {
		sessionDesignsByID[strings.TrimSpace(design.ID)] = design
	}
	return sessionDesignsByID, nil
}

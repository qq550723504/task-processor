package listingkit

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

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

func (s *taskStudioBatchService) completeStudioBatchTaskExecution(
	ctx context.Context,
	batchID string,
	session *SheinStudioSession,
	batch *StudioBatchRecord,
	createdTasks []SheinStudioCreatedTask,
	failedTasks []SheinStudioFailedTask,
) (*CreateStudioBatchTasksResult, error) {
	if sessionUpdater, ok := s.studioSessionRepo.(interface {
		UpdateSession(context.Context, *SheinStudioSession) error
	}); ok && session != nil {
		session.CreatedTasks = mergeStudioCreatedTasks(session.CreatedTasks, createdTasks)
		session.CreatedTaskIDs = buildCreatedTaskIDs(session.CreatedTasks)
		session.FailedTasks = append(SheinStudioFailedTaskList(nil), failedTasks...)
		session.UpdatedAt = s.currentTime().UTC()
		if err := sessionUpdater.UpdateSession(ctx, session); err != nil {
			return nil, err
		}
	}
	if len(createdTasks) > 0 && batch != nil {
		batch.Status = StudioBatchStatusTasksCreated
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
		Batch:        detail.Batch,
		Items:        detail.Items,
		CreatedTasks: createdTasks,
		FailedTasks:  failedTasks,
	}, nil
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
) ([]listingStudioBatchTaskExecuteCandidate, *SheinStudioSession, error) {
	designIDs, session, batchDetail, err := s.prepareStudioBatchTaskCreation(ctx, batchID, &CreateStudioBatchTasksRequest{
		DesignIDs: append([]string(nil), designIDs...),
	})
	if err != nil {
		return nil, nil, err
	}
	itemByID := make(map[string]StudioBatchItemRecord, len(batchDetail.Items))
	for _, item := range batchDetail.Items {
		itemByID[item.ID] = item
	}
	designs, err := s.repo.ListStudioMaterializedDesignsByIDs(ctx, batchID, designIDs)
	if err != nil {
		return nil, nil, err
	}
	sessionDesignsByID, err := s.loadStudioBatchSessionDesigns(ctx, batchID)
	if err != nil {
		return nil, nil, err
	}
	candidates := make([]listingStudioBatchTaskExecuteCandidate, 0, len(designs))
	for _, design := range designs {
		item, ok := itemByID[design.ItemID]
		if !ok {
			return nil, nil, gorm.ErrRecordNotFound
		}
		selections := resolveStudioBatchItemSelections(batchDetail.Batch, item)
		if len(selections) == 0 {
			selections = []SheinStudioGroupedSelection{{
				SelectionID:  selectionIDForStudioSelection(SheinStudioSelection(session.Selection)),
				Selection:    SheinStudioSelection(session.Selection),
				Eligible:     true,
				SheinStoreID: strings.TrimSpace(session.SheinStoreID),
			}}
		}
		for _, grouped := range selections {
			candidates = append(candidates, listingStudioBatchTaskExecuteCandidate{
				session: session,
				design:  design,
				grouped: grouped,
				title: firstNonEmpty(
					strings.TrimSpace(grouped.Selection.VariantLabel),
					strings.TrimSpace(grouped.Selection.ProductName),
					strings.TrimSpace(design.TargetGroupLabel),
					strings.TrimSpace(design.ID),
				),
				sessionDesign: sessionDesignsByID[design.ID],
			})
		}
	}
	return candidates, session, nil
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
	for _, design := range designs {
		if design.ReviewStatus != StudioMaterializedDesignReviewStatusApproved {
			return nil, nil, nil, NewStudioBatchActionValidationError(fmt.Sprintf("design %s is not approved", design.ID))
		}
	}
	session, err := s.loadStudioBatchTaskSession(ctx, batchID)
	if err != nil {
		return nil, nil, nil, err
	}
	return designIDs, session, batchDetail, nil
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

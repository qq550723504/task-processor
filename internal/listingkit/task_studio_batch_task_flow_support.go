package listingkit

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func (s *taskStudioBatchService) resumeStudioBatchTaskCreation(ctx context.Context, batchID string) (*CreateStudioBatchTasksResult, error) {
	session, err := s.loadStudioBatchTaskSession(ctx, batchID)
	if err != nil {
		return nil, err
	}
	designIDs := normalizeStudioBatchDesignIDs([]string(session.PendingTaskDesignIDs))
	if len(designIDs) == 0 {
		detail, detailErr := s.GetStudioBatchDetail(ctx, batchID)
		if detailErr != nil {
			return nil, detailErr
		}
		return &CreateStudioBatchTasksResult{
			Batch:        detail.Batch,
			Items:        detail.Items,
			CreatedTasks: detail.CreatedTasks,
			FailedTasks:  detail.FailedTasks,
		}, nil
	}
	result, err := s.CreateStudioBatchTasks(ctx, batchID, &CreateStudioBatchTasksRequest{DesignIDs: designIDs})
	if err != nil {
		return nil, err
	}
	if sessionUpdater, ok := s.studioSessionRepo.(interface {
		UpdateSession(context.Context, *SheinStudioSession) error
	}); ok {
		session.PendingTaskDesignIDs = nil
		session.CreatedTasks = append(SheinStudioCreatedTaskList(nil), result.CreatedTasks...)
		session.CreatedTaskIDs = buildCreatedTaskIDs(result.CreatedTasks)
		session.FailedTasks = append(SheinStudioFailedTaskList(nil), result.FailedTasks...)
		session.Status = SheinStudioSessionStatusTasksCreated
		session.UpdatedAt = s.currentTime().UTC()
		if err := sessionUpdater.UpdateSession(ctx, session); err != nil {
			return nil, err
		}
	}
	batch, err := s.repo.GetStudioBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	batch.Status = StudioBatchStatusTasksCreated
	batch.UpdatedAt = s.currentTime().UTC()
	if err := s.repo.UpdateStudioBatch(ctx, batch); err != nil {
		return nil, err
	}
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

package listingkit

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

func resolveStudioBatchDetailWithoutGraph(ctx context.Context, studioSessionRepo studioBatchSeedSessionRepository, batchID string) (*StudioBatchDetail, bool, error) {
	if studioSessionRepo == nil {
		return nil, false, gorm.ErrRecordNotFound
	}
	session, err := studioSessionRepo.GetSession(ctx, batchID)
	if err != nil {
		return nil, false, err
	}
	if session == nil || !session.SavedAsBatch {
		return nil, false, ErrStudioSessionNotFound
	}
	if shouldSyncStudioBatchGraphOnRead(session) {
		return nil, true, nil
	}
	return buildStudioBatchDraftOnlyDetail(session), false, nil
}

func projectStudioBatchDetail(
	detail *StudioBatchDetailGraph,
	draftUpdatedAt *time.Time,
	createdTasks []SheinStudioCreatedTask,
	failedTasks []SheinStudioFailedTask,
) *StudioBatchDetail {
	if detail == nil {
		return &StudioBatchDetail{}
	}

	batch := projectStudioBatchRecord(detail.Batch, detail.Items, draftUpdatedAt)
	items := make([]StudioBatchItemDetail, 0, len(detail.Items))
	for _, item := range detail.Items {
		items = append(items, StudioBatchItemDetail{
			Item:     item,
			Attempts: append([]StudioGenerationAttemptRecord(nil), detail.AttemptsByItem[item.ID]...),
			Designs:  append([]StudioMaterializedDesignRecord(nil), detail.DesignsByItem[item.ID]...),
		})
	}

	return &StudioBatchDetail{
		Batch:        batch,
		Items:        items,
		CreatedTasks: append([]SheinStudioCreatedTask(nil), createdTasks...),
		FailedTasks:  append([]SheinStudioFailedTask(nil), failedTasks...),
	}
}

func projectStudioBatchRecord(batch *StudioBatchRecord, items []StudioBatchItemRecord, draftUpdatedAt *time.Time) *StudioBatchRecord {
	if batch == nil {
		return nil
	}
	cloned := *batch
	if cloned.Status != StudioBatchStatusTasksCreated {
		cloned.Status = aggregateStudioBatchStatus(items)
	}
	cloned.DraftUpdatedAt = draftUpdatedAt
	return &cloned
}

func loadStudioBatchDraftState(ctx context.Context, studioSessionRepo studioBatchSeedSessionRepository, batchID string) (*time.Time, []SheinStudioCreatedTask, []SheinStudioFailedTask, error) {
	if studioSessionRepo == nil {
		return nil, nil, nil, nil
	}
	session, err := studioSessionRepo.GetSession(ctx, batchID)
	switch {
	case err == nil:
		if session == nil || !session.SavedAsBatch {
			return nil, nil, nil, nil
		}
		updatedAt := session.UpdatedAt.UTC()
		return &updatedAt, append([]SheinStudioCreatedTask(nil), session.CreatedTasks...), append([]SheinStudioFailedTask(nil), session.FailedTasks...), nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, nil, nil, nil
	default:
		return nil, nil, nil, err
	}
}

func shouldSyncStudioBatchGraphOnRead(session *SheinStudioSession) bool {
	if session == nil {
		return false
	}
	if session.Status == SheinStudioSessionStatusGenerating {
		return true
	}
	if session.GenerationJobID != "" {
		return true
	}
	return len(session.GenerationJobs) > 0
}

func buildStudioBatchDraftOnlyDetail(session *SheinStudioSession) *StudioBatchDetail {
	if session == nil {
		return &StudioBatchDetail{}
	}
	batch := buildStudioBatchRecordFromSessionDraft(session, session.UpdatedAt.UTC())
	batch.Status = StudioBatchStatusDraft
	updatedAt := session.UpdatedAt.UTC()
	batch.DraftUpdatedAt = &updatedAt
	return &StudioBatchDetail{
		Batch:        batch,
		Items:        []StudioBatchItemDetail{},
		CreatedTasks: append([]SheinStudioCreatedTask(nil), session.CreatedTasks...),
		FailedTasks:  append([]SheinStudioFailedTask(nil), session.FailedTasks...),
	}
}

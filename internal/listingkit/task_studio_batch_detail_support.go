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

	projected := &StudioBatchDetail{Batch: batch, Items: items}
	projected.StatusGroups = BuildStudioBatchStatusGroups(projected)
	return projected
}

func projectStudioBatchRecord(batch *StudioBatchRecord, items []StudioBatchItemRecord, draftUpdatedAt *time.Time) *StudioBatchRecord {
	if batch == nil {
		return nil
	}
	cloned := *batch
	cloned.Status = resolveProjectedStudioBatchStatus(cloned.Status, items)
	cloned.DraftUpdatedAt = draftUpdatedAt
	return &cloned
}

func loadStudioBatchDraftState(
	ctx context.Context,
	studioSessionRepo studioBatchSeedSessionRepository,
	batchID string,
) (*time.Time, error) {
	var session *SheinStudioSession
	if studioSessionRepo != nil {
		loaded, err := studioSessionRepo.GetSession(ctx, batchID)
		switch {
		case err == nil:
			session = loaded
		case errors.Is(err, gorm.ErrRecordNotFound):
		case err != nil:
			return nil, err
		}
	}
	if studioSessionRepo == nil || session == nil {
		return nil, nil
	}
	if !session.SavedAsBatch {
		return nil, nil
	}
	updatedAt := session.UpdatedAt.UTC()
	return &updatedAt, nil
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
	detail := &StudioBatchDetail{
		Batch: batch,
		Items: []StudioBatchItemDetail{},
	}
	detail.StatusGroups = BuildStudioBatchStatusGroups(detail)
	return detail
}

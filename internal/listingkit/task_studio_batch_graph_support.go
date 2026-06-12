package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

func (s *taskStudioBatchService) syncStudioBatchRetryExecutionConfigFromDraft(ctx context.Context, batchID string) error {
	if s == nil || s.repo == nil || s.studioSessionRepo == nil {
		return nil
	}

	session, err := s.studioSessionRepo.GetSession(ctx, batchID)
	if err != nil {
		return err
	}
	if session == nil || !session.SavedAsBatch {
		return nil
	}

	batch, err := s.repo.GetStudioBatch(ctx, batchID)
	if err != nil {
		return err
	}
	if batch == nil {
		return nil
	}

	batch.Prompt = session.Prompt
	batch.StyleCount = session.StyleCount
	batch.VariationIntensity = session.VariationIntensity
	batch.ArtworkModel = session.ArtworkModel
	batch.SelectedSDSImages = append(SheinStudioSelectedSDSImageList(nil), session.SelectedSDSImages...)
	batch.TransparentBackground = session.TransparentBackground
	if storeID, convErr := strconv.ParseInt(strings.TrimSpace(session.SheinStoreID), 10, 64); convErr == nil {
		batch.SheinStoreID = storeID
	}
	batch.UpdatedAt = s.currentTime().UTC()
	return s.repo.UpdateStudioBatch(ctx, batch)
}

func (s *taskStudioBatchService) refreshStudioBatchGenerationGraph(ctx context.Context, batchID string) error {
	if s.studioSessionRepo == nil {
		return fmt.Errorf("studio session repository is not configured")
	}
	session, err := s.studioSessionRepo.GetSession(ctx, batchID)
	if err != nil {
		return err
	}
	if session == nil || !session.SavedAsBatch {
		return ErrStudioSessionNotFound
	}

	_, existingErr := s.repo.GetStudioBatch(ctx, batchID)
	if existingErr != nil && !errors.Is(existingErr, gorm.ErrRecordNotFound) {
		return existingErr
	}

	now := s.currentTime().UTC()
	batch := buildStudioBatchRecordFromSessionDraft(session, now)
	items := expandStudioBatchItems(batch)
	for index := range items {
		items[index].CreatedAt = now.Add(time.Duration(index) * time.Second)
		items[index].UpdatedAt = items[index].CreatedAt
	}
	if errors.Is(existingErr, gorm.ErrRecordNotFound) {
		return s.repo.CreateStudioBatchGraph(ctx, batch, items, nil, nil)
	}
	return s.repo.ReplaceStudioBatchGenerationGraph(ctx, batch, items)
}

func (s *taskStudioBatchService) ensureStudioBatchGenerationGraphForResume(ctx context.Context, batchID string) error {
	if s.repo == nil {
		return fmt.Errorf("studio batch repository is not configured")
	}

	_, err := s.repo.GetStudioBatch(ctx, batchID)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return s.refreshStudioBatchGenerationGraph(ctx, batchID)
	default:
		return err
	}
}

func buildStudioBatchRecordFromSessionDraft(session *SheinStudioSession, now time.Time) *StudioBatchRecord {
	batch := &StudioBatchRecord{
		ID:                    session.ID,
		Status:                StudioBatchStatusGenerating,
		Prompt:                session.Prompt,
		GroupedImageMode:      strings.TrimSpace(session.GroupedImageMode),
		Selection:             session.Selection,
		GroupedSelections:     append(SheinStudioGroupedSelectionList(nil), session.GroupedSelections...),
		StyleCount:            session.StyleCount,
		VariationIntensity:    session.VariationIntensity,
		ArtworkModel:          session.ArtworkModel,
		SelectedSDSImages:     append(SheinStudioSelectedSDSImageList(nil), session.SelectedSDSImages...),
		TransparentBackground: session.TransparentBackground,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if storeID, convErr := strconv.ParseInt(strings.TrimSpace(session.SheinStoreID), 10, 64); convErr == nil {
		batch.SheinStoreID = storeID
	}
	if groupedMode := strings.TrimSpace(session.GroupedImageMode); groupedMode == "per_product" || groupedMode == "shared_by_size" {
		batch.GroupedImageMode = groupedMode
	}
	if batch.GroupedImageMode == "" {
		batch.GroupedImageMode = "shared_by_size"
	}
	return batch
}

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

	draftBatch := buildStudioBatchRecordFromSessionDraft(session, s.currentTime().UTC())
	if err := validateStudioBatchRecordDesignSource(draftBatch); err != nil {
		if validateStudioBatchRecordDesignSource(batch) == nil {
			return nil
		}
		return err
	}

	batch.Prompt = session.Prompt
	batch.StyleCount = session.StyleCount
	batch.VariationIntensity = session.VariationIntensity
	batch.ArtworkModel = session.ArtworkModel
	batch.SelectedSDSImages = append(SheinStudioSelectedSDSImageList(nil), session.SelectedSDSImages...)
	batch.HotStyleReferenceImageURLs = append(SheinStudioStringList(nil), session.HotStyleReferenceImageURLs...)
	batch.HotStyleReferenceBrief = session.HotStyleReferenceBrief
	batch.HotStyleReferencePrompt = session.HotStyleReferencePrompt
	batch.TransparentBackground = session.TransparentBackground
	if storeID, convErr := strconv.ParseInt(strings.TrimSpace(session.SheinStoreID), 10, 64); convErr == nil {
		batch.SheinStoreID = storeID
	}
	batch.UpdatedAt = s.currentTime().UTC()
	return s.repo.UpdateStudioBatch(ctx, batch)
}

func (s *taskStudioBatchService) refreshStudioBatchGenerationGraph(ctx context.Context, batchID string) error {
	return refreshStudioBatchGenerationGraph(ctx, s.repo, s.studioSessionRepo, s.currentTime, batchID)
}

func (s *taskStudioBatchService) ensureStudioBatchGenerationGraphForResume(ctx context.Context, batchID string) error {
	return ensureStudioBatchGenerationGraphForResume(ctx, s.repo, s.studioSessionRepo, s.currentTime, batchID)
}

func refreshStudioBatchGenerationGraph(
	ctx context.Context,
	repo StudioBatchRepository,
	studioSessionRepo studioBatchSeedSessionRepository,
	currentTime func() time.Time,
	batchID string,
) error {
	if studioSessionRepo == nil {
		return fmt.Errorf("studio session repository is not configured")
	}
	session, err := studioSessionRepo.GetSession(ctx, batchID)
	if err != nil {
		return err
	}
	if session == nil || !session.SavedAsBatch {
		return ErrStudioSessionNotFound
	}

	_, existingErr := repo.GetStudioBatch(ctx, batchID)
	if existingErr != nil && !errors.Is(existingErr, gorm.ErrRecordNotFound) {
		return existingErr
	}

	now := currentTime().UTC()
	batch := buildStudioBatchRecordFromSessionDraft(session, now)
	if err := validateStudioBatchRecordDesignSource(batch); err != nil {
		if existingErr == nil {
			existing, getErr := repo.GetStudioBatch(ctx, batchID)
			if getErr != nil {
				return getErr
			}
			if validateStudioBatchRecordDesignSource(existing) == nil {
				return nil
			}
		}
		return err
	}
	items := expandStudioBatchItems(batch)
	for index := range items {
		items[index].CreatedAt = now.Add(time.Duration(index) * time.Second)
		items[index].UpdatedAt = items[index].CreatedAt
	}
	if errors.Is(existingErr, gorm.ErrRecordNotFound) {
		return repo.CreateStudioBatchGraph(ctx, batch, items, nil, nil)
	}
	return repo.ReplaceStudioBatchGenerationGraph(ctx, batch, items)
}

func ensureStudioBatchGenerationGraphForResume(
	ctx context.Context,
	repo StudioBatchRepository,
	studioSessionRepo studioBatchSeedSessionRepository,
	currentTime func() time.Time,
	batchID string,
) error {
	if repo == nil {
		return fmt.Errorf("studio batch repository is not configured")
	}

	_, err := repo.GetStudioBatch(ctx, batchID)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return refreshStudioBatchGenerationGraph(ctx, repo, studioSessionRepo, currentTime, batchID)
	default:
		return err
	}
}

func buildStudioBatchRecordFromSessionDraft(session *SheinStudioSession, now time.Time) *StudioBatchRecord {
	batch := &StudioBatchRecord{
		ID:                         session.ID,
		Status:                     StudioBatchStatusGenerating,
		Prompt:                     session.Prompt,
		PromptMode:                 strings.TrimSpace(session.PromptMode),
		GroupedImageMode:           strings.TrimSpace(session.GroupedImageMode),
		Selection:                  session.Selection,
		GroupedSelections:          append(SheinStudioGroupedSelectionList(nil), session.GroupedSelections...),
		StyleCount:                 session.StyleCount,
		VariationIntensity:         session.VariationIntensity,
		ArtworkModel:               session.ArtworkModel,
		SelectedSDSImages:          append(SheinStudioSelectedSDSImageList(nil), session.SelectedSDSImages...),
		HotStyleReferenceImageURLs: append(SheinStudioStringList(nil), session.HotStyleReferenceImageURLs...),
		HotStyleReferenceBrief:     session.HotStyleReferenceBrief,
		HotStyleReferencePrompt:    session.HotStyleReferencePrompt,
		TransparentBackground:      session.TransparentBackground,
		CreatedAt:                  now,
		UpdatedAt:                  now,
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

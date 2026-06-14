package listingkit

import (
	"context"
	"errors"

	studiodomain "task-processor/internal/listing/studio"
)

type listingStudioBatchDraftRunner = studiodomain.BatchDraftService[
	SheinStudioSession,
	SheinStudioDesign,
	SheinStudioSessionGalleryItem,
	SheinStudioBatchListItem,
]

func newListingStudioBatchDraftService(repo studioBatchDraftRepository) *listingStudioBatchDraftRunner {
	return studiodomain.NewBatchDraftService(studiodomain.BatchDraftServiceConfig[
		SheinStudioSession,
		SheinStudioDesign,
		SheinStudioSessionGalleryItem,
		SheinStudioBatchListItem,
	]{
		Repo: studioBatchDraftRepositoryAdapter{repo: repo},
		IsSavedBatch: func(session *SheinStudioSession) bool {
			return session != nil && session.SavedAsBatch
		},
		SessionID: func(session *SheinStudioSession) string {
			if session == nil {
				return ""
			}
			return session.ID
		},
		MapBatchListItem: mapStudioBatchListItem,
	})
}

type studioBatchDraftRepositoryAdapter struct {
	repo studioBatchDraftRepository
}

func (a studioBatchDraftRepositoryAdapter) GetSession(ctx context.Context, sessionID string) (*SheinStudioSession, error) {
	if a.repo == nil {
		return nil, nil
	}
	return a.repo.GetSession(ctx, sessionID)
}

func (a studioBatchDraftRepositoryAdapter) DeleteSession(ctx context.Context, sessionID string) error {
	if a.repo == nil {
		return nil
	}
	return a.repo.DeleteSession(ctx, sessionID)
}

func (a studioBatchDraftRepositoryAdapter) ListSessionDesigns(ctx context.Context, sessionID string) ([]SheinStudioDesign, error) {
	if a.repo == nil {
		return nil, nil
	}
	return a.repo.ListSessionDesigns(ctx, sessionID)
}

func (a studioBatchDraftRepositoryAdapter) CountSessionDesignsBySessionIDs(ctx context.Context, sessionIDs []string) (map[string]int, error) {
	if a.repo == nil {
		return nil, nil
	}
	return a.repo.CountSessionDesignsBySessionIDs(ctx, sessionIDs)
}

func (a studioBatchDraftRepositoryAdapter) ListGalleryItems(ctx context.Context, limit int) ([]SheinStudioSessionGalleryItem, error) {
	if a.repo == nil {
		return nil, nil
	}
	return a.repo.ListGalleryItems(ctx, limit)
}

func (a studioBatchDraftRepositoryAdapter) ListBatchSessions(ctx context.Context, limit int) ([]SheinStudioSession, error) {
	if a.repo == nil {
		return nil, nil
	}
	return a.repo.ListBatchSessions(ctx, limit)
}

func adaptStudioBatchDraftError(err error) error {
	if errors.Is(err, studiodomain.ErrBatchDraftNotFound) {
		return ErrStudioSessionNotFound
	}
	return err
}

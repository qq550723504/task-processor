package studio

import (
	"context"
	"errors"
	"fmt"
)

var ErrBatchDraftNotFound = errors.New("studio batch draft not found")

type BatchDraftDetail[Session any, Design any] struct {
	Batch   *Session
	Designs []Design
}

type BatchDraftList[Item any] struct {
	Items []Item
	Total int
}

type BatchDraftRepository[Session any, Design any, GalleryItem any] interface {
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
	ListSessionDesigns(ctx context.Context, sessionID string) ([]Design, error)
	CountSessionDesignsBySessionIDs(ctx context.Context, sessionIDs []string) (map[string]int, error)
	ListGalleryItems(ctx context.Context, limit int) ([]GalleryItem, error)
	ListBatchSessions(ctx context.Context, limit int) ([]Session, error)
}

type BatchDraftService[Session any, Design any, GalleryItem any, BatchListItem any] struct {
	repo             BatchDraftRepository[Session, Design, GalleryItem]
	isSavedBatch     func(*Session) bool
	sessionID        func(*Session) string
	mapBatchListItem func(*Session, int) BatchListItem
}

type BatchDraftServiceConfig[Session any, Design any, GalleryItem any, BatchListItem any] struct {
	Repo             BatchDraftRepository[Session, Design, GalleryItem]
	IsSavedBatch     func(*Session) bool
	SessionID        func(*Session) string
	MapBatchListItem func(*Session, int) BatchListItem
}

func NewBatchDraftService[Session any, Design any, GalleryItem any, BatchListItem any](config BatchDraftServiceConfig[Session, Design, GalleryItem, BatchListItem]) *BatchDraftService[Session, Design, GalleryItem, BatchListItem] {
	return &BatchDraftService[Session, Design, GalleryItem, BatchListItem]{
		repo:             config.Repo,
		isSavedBatch:     config.IsSavedBatch,
		sessionID:        config.SessionID,
		mapBatchListItem: config.MapBatchListItem,
	}
}

func (s *BatchDraftService[Session, Design, GalleryItem, BatchListItem]) ListSessionGallery(ctx context.Context, limit int) (*BatchDraftList[GalleryItem], error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("studio batch draft repository is not configured")
	}
	items, err := s.repo.ListGalleryItems(ctx, limit)
	if err != nil {
		return nil, err
	}
	return &BatchDraftList[GalleryItem]{
		Items: items,
		Total: len(items),
	}, nil
}

func (s *BatchDraftService[Session, Design, GalleryItem, BatchListItem]) ListBatches(ctx context.Context, limit int) (*BatchDraftList[BatchListItem], error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("studio batch draft repository is not configured")
	}
	if s.sessionID == nil || s.mapBatchListItem == nil {
		return nil, fmt.Errorf("studio batch draft list mapping is not configured")
	}
	sessions, err := s.repo.ListBatchSessions(ctx, limit)
	if err != nil {
		return nil, err
	}
	sessionIDs := make([]string, 0, len(sessions))
	for i := range sessions {
		if id := s.sessionID(&sessions[i]); id != "" {
			sessionIDs = append(sessionIDs, id)
		}
	}
	designCounts, err := s.repo.CountSessionDesignsBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, err
	}
	items := make([]BatchListItem, 0, len(sessions))
	for i := range sessions {
		session := &sessions[i]
		items = append(items, s.mapBatchListItem(session, designCounts[s.sessionID(session)]))
	}
	return &BatchDraftList[BatchListItem]{
		Items: items,
		Total: len(items),
	}, nil
}

func (s *BatchDraftService[Session, Design, GalleryItem, BatchListItem]) GetBatch(ctx context.Context, batchID string) (*BatchDraftDetail[Session, Design], error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("studio batch draft repository is not configured")
	}
	session, err := s.repo.GetSession(ctx, batchID)
	if err != nil {
		return nil, err
	}
	if session == nil || (s.isSavedBatch != nil && !s.isSavedBatch(session)) {
		return nil, ErrBatchDraftNotFound
	}
	designs, err := s.repo.ListSessionDesigns(ctx, s.sessionID(session))
	if err != nil {
		return nil, err
	}
	return &BatchDraftDetail[Session, Design]{
		Batch:   session,
		Designs: designs,
	}, nil
}

func (s *BatchDraftService[Session, Design, GalleryItem, BatchListItem]) DeleteBatch(ctx context.Context, batchID string) error {
	if s == nil || s.repo == nil {
		return fmt.Errorf("studio batch draft repository is not configured")
	}
	session, err := s.repo.GetSession(ctx, batchID)
	if err != nil {
		return err
	}
	if session == nil || (s.isSavedBatch != nil && !s.isSavedBatch(session)) {
		return nil
	}
	return s.repo.DeleteSession(ctx, batchID)
}

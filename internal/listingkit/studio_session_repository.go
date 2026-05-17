package listingkit

import "context"

type StudioSessionRepository interface {
	FindLatestSessionBySelectionKey(ctx context.Context, selectionKey string) (*SheinStudioSession, error)
	CreateSession(ctx context.Context, session *SheinStudioSession) error
	GetSession(ctx context.Context, sessionID string) (*SheinStudioSession, error)
	UpdateSession(ctx context.Context, session *SheinStudioSession) error
	DeleteSession(ctx context.Context, sessionID string) error
	ReplaceDesigns(ctx context.Context, sessionID string, approvedIDs []string, designs []SheinStudioDesign) error
	ListSessionDesigns(ctx context.Context, sessionID string) ([]SheinStudioDesign, error)
	ListGalleryItems(ctx context.Context, limit int) ([]SheinStudioSessionGalleryItem, error)
	ListBatchSessions(ctx context.Context, limit int) ([]SheinStudioSession, error)
}

package listingkit

import (
	"context"
	"time"

	"task-processor/internal/shared/tenantctx"
)

type StudioBatchTaskLinkRepository interface {
	GetStudioBatchTaskLinkByCandidateKey(ctx context.Context, candidateKey string) (*StudioBatchTaskLinkRecord, error)
	CreateStudioBatchTaskLink(ctx context.Context, link *StudioBatchTaskLinkRecord) error
	UpdateStudioBatchTaskLink(ctx context.Context, link *StudioBatchTaskLinkRecord) error
	ListStudioBatchTaskLinksByBatchID(ctx context.Context, batchID string) ([]StudioBatchTaskLinkRecord, error)
	ClaimStudioBatchTaskCandidate(ctx context.Context, candidateKey string, fromStatus string, toStatus string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error)
	ClaimStudioBatchTaskCandidateUpdatedAt(ctx context.Context, candidateKey string, fromStatus string, observedUpdatedAt time.Time, toStatus string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error)
}

func applyStudioBatchTaskLinkCreateScope(ctx context.Context, link *StudioBatchTaskLinkRecord) {
	if link == nil {
		return
	}
	link.TenantID = tenantctx.TenantIDFromContext(ctx)
	link.UserID = RequestUserIDFromContext(ctx)
}

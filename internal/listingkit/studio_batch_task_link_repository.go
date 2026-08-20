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

// studioBatchTaskLinkLeaseRepository is implemented by repositories that can
// atomically claim and refresh a creating link with an ownership token.
// Keeping it separate preserves compatibility for read/backfill-only callers;
// task creation fails closed when the production repository lacks this lease
// contract.
type studioBatchTaskLinkLeaseRepository interface {
	ClaimStudioBatchTaskCandidateWithToken(ctx context.Context, candidateKey string, fromStatus string, toStatus string, claimToken string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error)
	ClaimStudioBatchTaskCandidateUpdatedAtWithToken(ctx context.Context, candidateKey string, fromStatus string, observedUpdatedAt time.Time, toStatus string, claimToken string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error)
	RefreshStudioBatchTaskLink(ctx context.Context, candidateKey string, claimToken string, updatedAt time.Time) (bool, error)
}

func applyStudioBatchTaskLinkCreateScope(ctx context.Context, link *StudioBatchTaskLinkRecord) {
	if link == nil {
		return
	}
	link.TenantID = tenantctx.TenantIDFromContext(ctx)
	link.UserID = RequestUserIDFromContext(ctx)
}

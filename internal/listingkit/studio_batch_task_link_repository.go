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
	UpdateStudioBatchTaskLinkWithClaimToken(ctx context.Context, link *StudioBatchTaskLinkRecord, claimToken string) (bool, error)
}

// studioBatchTaskLinkReclaimRepository atomically records the replacement
// claim and the previous reservation token. This closes the crash window
// between a successful stale reclaim and a follow-up cleanup write.
type studioBatchTaskLinkReclaimRepository interface {
	ClaimStudioBatchTaskCandidateUpdatedAtWithTokenAndPendingRelease(ctx context.Context, candidateKey string, fromStatus string, observedUpdatedAt time.Time, toStatus string, claimToken string, pendingReleaseClaimToken string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error)
}

// studioBatchTaskLinkUsageSettlementRepository atomically elects the one
// caller allowed to perform the non-idempotent legacy usage settlement.
type studioBatchTaskLinkUsageSettlementRepository interface {
	ClaimStudioBatchProductImageUsageSettled(ctx context.Context, candidateKey string, updatedAt time.Time) (bool, error)
}

// studioBatchTaskLinkProductImageUsageRouteRepository atomically finalizes
// the accounting route while the creating worker owns the candidate lease.
type studioBatchTaskLinkProductImageUsageRouteRepository interface {
	ResolveStudioBatchProductImageUsageRoute(ctx context.Context, candidateKey string, claimToken string, route studioBatchProductImageUsageRoute, updatedAt time.Time) (stored studioBatchProductImageUsageRoute, changed bool, err error)
}

// studioBatchTaskLinkProductImageUsageCompatibilityRouteRepository atomically
// records the route selected for links created before route persistence was
// introduced. It never overwrites a route chosen by another lifecycle worker.
type studioBatchTaskLinkProductImageUsageCompatibilityRouteRepository interface {
	ResolveStudioBatchProductImageUsageCompatibilityRoute(ctx context.Context, candidateKey string, route studioBatchProductImageUsageRoute, updatedAt time.Time) (stored studioBatchProductImageUsageRoute, changed bool, err error)
}

func applyStudioBatchTaskLinkCreateScope(ctx context.Context, link *StudioBatchTaskLinkRecord) {
	if link == nil {
		return
	}
	link.TenantID = tenantctx.TenantIDFromContext(ctx)
	link.UserID = RequestUserIDFromContext(ctx)
}

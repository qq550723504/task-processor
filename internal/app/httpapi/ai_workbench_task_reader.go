package httpapi

import (
	"context"
	"errors"
	"task-processor/internal/agent"
	"task-processor/internal/aiworkbench"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/review"
	"time"
)

func (a *aiWorkbenchApplication) taskProjectionReader() aiworkbench.AuthorizedTaskProjectionReader {
	return aiworkbench.AuthorizedTaskProjectionReader{Now: time.Now, Authorize: func(ctx context.Context, scope aiworkbench.Scope) error {
		i, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
		if !ok || i.TenantID != scope.OrganizationID || i.UserID != scope.ActorID || !time.Now().Before(i.TokenExpiresAt) {
			return aiworkbench.ErrNotFound
		}
		return nil
	}, LookupRun: func(ctx context.Context, scope agent.Scope, binding agent.Binding, key string) (agent.Record, bool, error) {
		return a.agent.store.Lookup(ctx, scope, binding, key)
	}, ReadReview: func(ctx context.Context, runID string) (aiworkbench.TaskReviewState, bool, error) {
		r, found, e := a.agent.reviews.FindAgentReview(ctx, runID)
		if errors.Is(e, review.ErrForbidden) {
			m, f, e := a.agent.reviews.FindAgentTaskReviewMetadata(ctx, runID)
			return aiworkbench.TaskReviewState{State: m.State, Revision: m.Revision, OccurredAt: m.OccurredAt}, f, e
		}
		m := review.Record{State: r.State, Revision: r.Revision, History: r.History, Receipt: r.Receipt}.TaskStateMetadata()
		return aiworkbench.TaskReviewState{State: m.State, ID: r.ID, Revision: m.Revision, OccurredAt: m.OccurredAt}, found, e
	}}
}

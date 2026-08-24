package listingkit

import (
	"context"

	"task-processor/internal/authidentity"
)

// AuthenticatedIdentity is retained as a compatibility alias for callers that
// still depend on the ListingKit facade.
type AuthenticatedIdentity = authidentity.AuthenticatedIdentity

// WithAuthenticatedIdentity attaches a normalized verified identity to ctx.
func WithAuthenticatedIdentity(ctx context.Context, identity AuthenticatedIdentity) context.Context {
	return authidentity.WithAuthenticatedIdentity(ctx, identity)
}

// AuthenticatedIdentityFromContext returns the verified identity stored in ctx.
func AuthenticatedIdentityFromContext(ctx context.Context) (AuthenticatedIdentity, bool) {
	return authidentity.AuthenticatedIdentityFromContext(ctx)
}

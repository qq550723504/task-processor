package listingkit

import (
	"context"

	"task-processor/internal/authidentity"
)

func DetachedRequestContext(ctx context.Context) context.Context {
	detached := WithTenantID(context.Background(), TenantIDFromContext(ctx))
	detached = WithRequestIdentity(detached, RequestIdentityFromContext(ctx))
	detached = WithRequestRoles(detached, RequestRolesFromContext(ctx))
	detached = WithRequestTrace(detached, RequestTraceFromContext(ctx))
	if identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx); ok {
		detached = authidentity.WithAuthenticatedIdentity(detached, identity)
	}
	return detached
}

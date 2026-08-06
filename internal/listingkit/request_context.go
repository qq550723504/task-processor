package listingkit

import (
	"context"
)

func DetachedRequestContext(ctx context.Context) context.Context {
	detached := WithTenantID(context.Background(), TenantIDFromContext(ctx))
	detached = WithRequestIdentity(detached, RequestIdentityFromContext(ctx))
	detached = WithRequestRoles(detached, RequestRolesFromContext(ctx))
	detached = WithRequestTrace(detached, RequestTraceFromContext(ctx))
	if identity, ok := AuthenticatedIdentityFromContext(ctx); ok {
		detached = WithAuthenticatedIdentity(detached, identity)
	}
	return detached
}

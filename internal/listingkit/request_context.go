package listingkit

import (
	"context"
)

func DetachedRequestContext(ctx context.Context) context.Context {
	detached := WithTenantID(context.Background(), TenantIDFromContext(ctx))
	detached = WithRequestIdentity(detached, RequestIdentityFromContext(ctx))
	detached = WithRequestRoles(detached, RequestRolesFromContext(ctx))
	return WithRequestTrace(detached, RequestTraceFromContext(ctx))
}

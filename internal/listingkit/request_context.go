package listingkit

import (
	"context"

	openaiclient "task-processor/internal/infra/clients/openai"
)

func DetachedRequestContext(ctx context.Context) context.Context {
	detached := WithTenantID(context.Background(), TenantIDFromContext(ctx))
	detached = openaiclient.WithIdentity(detached, openaiclient.IdentityFromContext(ctx))
	detached = WithRequestRoles(detached, RequestRolesFromContext(ctx))
	return WithRequestTrace(detached, RequestTraceFromContext(ctx))
}

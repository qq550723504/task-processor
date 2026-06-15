package listingkit

import (
	"context"

	"task-processor/internal/shared/tenantctx"
)

const DefaultTenantID = tenantctx.DefaultTenantID

func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return tenantctx.WithTenantID(ctx, tenantID)
}

func TenantIDFromContext(ctx context.Context) string {
	return tenantctx.TenantIDFromContext(ctx)
}

func TenantScopeFromContext(ctx context.Context) (string, bool) {
	return tenantctx.TenantScopeFromContext(ctx)
}
